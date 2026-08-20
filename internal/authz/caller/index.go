// Package caller 同步 Caller 资源并提供访问密钥查询索引
package caller

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/authz/conf"
	"github.com/lgc202/ingate/pkg/accesskey"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate/pkg/generated/informers/externalversions"
	gatewaylisters "github.com/lgc202/ingate/pkg/generated/listers/gateway/v1"
)

var (
	// ErrUnauthenticated 表示请求未携带当前有效的 Caller 访问密钥
	ErrUnauthenticated = errors.New("caller authentication failed")
	// ErrForbidden 表示 Caller 存在但没有访问目标 Route 的权限
	ErrForbidden = errors.New("caller is not authorized for route")
)

// Identity 是一次成功鉴权后写入请求记录的稳定归属
type Identity struct {
	CallerID    string
	AccessKeyID string
}

type credentialEntry struct {
	callerID string
	digest   []byte
	routes   map[string]struct{}
	expires  time.Time
}

// credentialIndex 是一次完整资源同步生成的只读鉴权视图
// 每次资源变化都会构造新实例后原子替换，流量线程不会读到半更新状态
type credentialIndex struct {
	byKeyID map[string]credentialEntry
}

// Index 使用 informer 维护访问密钥 ID 到 Caller 权限的不可变索引
type Index struct {
	factory     informers.SharedInformerFactory
	lister      gatewaylisters.CallerLister
	logger      *slog.Logger
	credentials atomic.Pointer[credentialIndex]
	ready       atomic.Bool
	started     chan struct{}
	done        chan struct{}
	cancel      context.CancelFunc
}

// NewIndex 创建 Caller 访问密钥索引
func NewIndex(apiServer *conf.Data_APIServer, logger *slog.Logger) (*Index, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags(apiServer.GetMaster(), apiServer.GetKubeconfig())
	if err != nil {
		return nil, fmt.Errorf("build API Server client config: %w", err)
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create API Server resource client: %w", err)
	}
	factory := informers.NewSharedInformerFactory(client, 0)
	callers := factory.Gateway().V1().Callers()
	index := &Index{
		factory: factory,
		lister:  callers.Lister(),
		logger:  logger,
		started: make(chan struct{}),
		done:    make(chan struct{}),
	}
	index.credentials.Store(&credentialIndex{byKeyID: map[string]credentialEntry{}})
	if _, err := callers.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(any) { index.rebuild() },
		UpdateFunc: func(any, any) { index.rebuild() },
		DeleteFunc: func(any) { index.rebuild() },
	}); err != nil {
		return nil, fmt.Errorf("register Caller informer handler: %w", err)
	}
	return index, nil
}

// Start 等待首次 Caller 列表同步后持续监听密钥和授权变化
func (i *Index) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	i.cancel = cancel
	close(i.started)
	defer close(i.done)
	defer cancel()
	defer i.factory.Shutdown()
	defer i.ready.Store(false)

	i.factory.Start(runCtx.Done())
	for resourceType, synced := range i.factory.WaitForCacheSync(runCtx.Done()) {
		if synced {
			continue
		}
		if runCtx.Err() != nil {
			return nil
		}
		return fmt.Errorf("sync Caller cache for %v", resourceType)
	}
	i.rebuild()
	i.ready.Store(true)
	<-runCtx.Done()
	return nil
}

// Stop 停止资源监听并等待 informer 退出
func (i *Index) Stop(ctx context.Context) error {
	select {
	case <-i.started:
	case <-i.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
	i.cancel()
	select {
	case <-i.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Ready 表示首次 Caller 列表已经同步完成
func (i *Index) Ready() bool {
	return i.ready.Load()
}

// Authorize 验证完整访问密钥并检查 Caller 是否有权访问 Route
func (i *Index) Authorize(value, routeID string, now time.Time) (Identity, error) {
	keyID, err := accesskey.KeyID(value)
	if err != nil {
		return Identity{}, ErrUnauthenticated
	}
	entry, exists := i.credentials.Load().byKeyID[keyID]
	if !exists {
		return Identity{}, ErrUnauthenticated
	}
	actual, _ := hex.DecodeString(accesskey.Digest(value))
	if subtle.ConstantTimeCompare(actual, entry.digest) != 1 {
		return Identity{}, ErrUnauthenticated
	}
	if !entry.expires.IsZero() && !now.Before(entry.expires) {
		return Identity{}, ErrUnauthenticated
	}
	identity := Identity{CallerID: entry.callerID, AccessKeyID: keyID}
	if _, allowed := entry.routes[routeID]; !allowed {
		// 密钥已经确认身份，只是缺少当前 Route 权限；保留身份用于记录越权请求
		return identity, ErrForbidden
	}
	return identity, nil
}

func (i *Index) rebuild() {
	callers, err := i.lister.List(labels.Everything())
	if err != nil {
		i.logger.Error("Caller index refresh failed", "error", err)
		return
	}
	credentials := make(map[string]credentialEntry)
	ambiguousKeyIDs := make(map[string]struct{})
	for _, caller := range callers {
		if !caller.Spec.Enabled {
			continue
		}
		routes := make(map[string]struct{}, len(caller.Spec.RouteRefs))
		for _, routeID := range caller.Spec.RouteRefs {
			routes[routeID] = struct{}{}
		}
		for _, key := range caller.Spec.AccessKeys {
			if !key.Enabled || !accesskey.ValidDigest(key.SecretDigest) {
				continue
			}
			if _, ambiguous := ambiguousKeyIDs[key.ID]; ambiguous {
				continue
			}
			if _, duplicate := credentials[key.ID]; duplicate {
				// 声明式 API 允许直接写资源；若外部调用方错误复用了密钥 ID，
				// 必须让该 ID 整体失效，不能依赖 informer 列表顺序选择某个 Caller
				delete(credentials, key.ID)
				ambiguousKeyIDs[key.ID] = struct{}{}
				continue
			}
			digest, _ := hex.DecodeString(key.SecretDigest)
			entry := credentialEntry{callerID: caller.Name, digest: digest, routes: routes}
			if key.ExpiresAt != nil {
				entry.expires = key.ExpiresAt.Time
			}
			credentials[key.ID] = entry
		}
	}
	i.credentials.Store(&credentialIndex{byKeyID: credentials})
	if len(ambiguousKeyIDs) > 0 {
		i.logger.Error("duplicate Caller access key IDs rejected", "count", len(ambiguousKeyIDs))
	}
}

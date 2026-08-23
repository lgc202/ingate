// Package apiserver 从声明式 API 同步 Authz 所需的 Caller 凭据
package apiserver

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/authz/biz"
	"github.com/lgc202/ingate/internal/authz/conf"
	"github.com/lgc202/ingate/internal/pkg/accesskey"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate/internal/pkg/generated/informers/externalversions"
	gatewaylisters "github.com/lgc202/ingate/internal/pkg/generated/listers/gateway/v1"
)

type credentialIndex struct {
	byKeyID map[string]biz.Credential
}

// CredentialCache 监听 Caller 资源并维护访问密钥 ID 到授权信息的只读索引
// 每次资源变化都会完整构造新索引后原子替换，流量线程不会读到半更新状态
type CredentialCache struct {
	factory         informers.SharedInformerFactory
	lister          gatewaylisters.CallerLister
	logger          *slog.Logger
	credentials     atomic.Pointer[credentialIndex]
	duplicateKeyIDs atomic.Int64
	ready           atomic.Bool
	started         chan struct{}
	done            chan struct{}
	cancel          context.CancelFunc
}

// NewCredentialCache 创建由 API Server 驱动的 Caller 凭据缓存
func NewCredentialCache(config *conf.Data_APIServer, logger *slog.Logger) (*CredentialCache, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags(config.GetMaster(), config.GetKubeconfig())
	if err != nil {
		return nil, fmt.Errorf("build API Server client config: %w", err)
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create API Server resource client: %w", err)
	}

	factory := informers.NewSharedInformerFactory(client, 0)
	callers := factory.Gateway().V1().Callers()
	credentialCache := &CredentialCache{
		factory: factory,
		lister:  callers.Lister(),
		logger:  logger,
		started: make(chan struct{}),
		done:    make(chan struct{}),
	}
	credentialCache.credentials.Store(&credentialIndex{byKeyID: map[string]biz.Credential{}})
	if _, err := callers.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(any) { credentialCache.rebuild() },
		UpdateFunc: func(any, any) { credentialCache.rebuild() },
		DeleteFunc: func(any) { credentialCache.rebuild() },
	}); err != nil {
		return nil, fmt.Errorf("register Caller informer handler: %w", err)
	}
	return credentialCache, nil
}

// Start 等待首次 Caller 列表同步后持续监听密钥和授权变化
func (c *CredentialCache) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	close(c.started)
	defer close(c.done)
	defer cancel()
	defer c.factory.Shutdown()
	defer c.ready.Store(false)

	c.factory.Start(runCtx.Done())
	for resourceType, synced := range c.factory.WaitForCacheSync(runCtx.Done()) {
		if synced {
			continue
		}
		if runCtx.Err() != nil {
			return nil
		}
		return fmt.Errorf("sync Caller cache for %v", resourceType)
	}
	c.rebuild()
	c.ready.Store(true)
	<-runCtx.Done()
	return nil
}

// Stop 停止 Caller 资源监听并等待 informer 退出
func (c *CredentialCache) Stop(ctx context.Context) error {
	select {
	case <-c.started:
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
	c.cancel()
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Ready 表示首次 Caller 列表已经同步完成
func (c *CredentialCache) Ready() bool {
	return c.ready.Load()
}

// Lookup 按公开访问密钥 ID 查询当前授权凭据
func (c *CredentialCache) Lookup(keyID string) (biz.Credential, bool) {
	credential, exists := c.credentials.Load().byKeyID[keyID]
	return credential, exists
}

func (c *CredentialCache) rebuild() {
	callers, err := c.lister.List(labels.Everything())
	if err != nil {
		c.logger.Error("refresh Caller credential cache failed", "err", err)
		return
	}
	credentials := make(map[string]biz.Credential)
	ambiguousKeyIDs := make(map[string]struct{})
	for _, caller := range callers {
		if !caller.Spec.Enabled {
			continue
		}
		routeIDs := make(map[string]struct{}, len(caller.Spec.RouteRefs))
		for _, routeID := range caller.Spec.RouteRefs {
			routeIDs[routeID] = struct{}{}
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
			credential := biz.Credential{
				CallerID: caller.Name,
				Digest:   key.SecretDigest,
				RouteIDs: routeIDs,
			}
			if key.ExpiresAt != nil {
				credential.ExpiresAt = key.ExpiresAt.Time
			}
			credentials[key.ID] = credential
		}
	}
	c.credentials.Store(&credentialIndex{byKeyID: credentials})
	duplicateCount := int64(len(ambiguousKeyIDs))
	previousCount := c.duplicateKeyIDs.Swap(duplicateCount)
	if duplicateCount > 0 && duplicateCount != previousCount {
		c.logger.Warn("duplicate Caller access key IDs rejected", "count", duplicateCount)
	} else if duplicateCount == 0 && previousCount > 0 {
		c.logger.Info("Caller access key ID conflicts resolved")
	}
}

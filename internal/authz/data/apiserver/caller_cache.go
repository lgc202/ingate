// Package apiserver 从声明式 API 同步 Authz 所需的 Caller 凭据。
package apiserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/lgc202/ingate/internal/authz/biz"
	"github.com/lgc202/ingate/internal/authz/conf"
	"github.com/lgc202/ingate/internal/pkg/accesskey"
	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/apiserverclient"
	"github.com/lgc202/ingate/internal/pkg/callerconfig"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate/internal/pkg/generated/informers/externalversions"
	gatewaylisters "github.com/lgc202/ingate/internal/pkg/generated/listers/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/version"
)

type credentialIndex struct {
	byKeyID map[string]biz.Credential
}

// CredentialCache 监听 Caller 资源并维护访问密钥 ID 到授权信息的只读索引
// 每次资源变化都会完整构造新索引后原子替换，流量线程不会读到半更新状态。
type CredentialCache struct {
	factory         informers.SharedInformerFactory
	lister          gatewaylisters.CallerLister
	logger          *slog.Logger
	credentials     atomic.Pointer[credentialIndex]
	duplicateKeyIDs atomic.Int64
	ready           atomic.Bool
	running         atomic.Bool
	done            chan struct{}

	lifecycleMu sync.Mutex
	cancel      context.CancelFunc
	stopping    bool
}

// NewCredentialCache 创建由 API Server 驱动的 Caller 凭据缓存。
func NewCredentialCache(config *conf.Data_APIServer, logger *slog.Logger) (*CredentialCache, error) {
	restConfig, err := apiserverclient.NewConfig(apiserverclient.Options{
		MasterURL:      config.GetMaster(),
		KubeconfigPath: config.GetKubeconfig(),
		BearerToken:    config.GetBearerToken(),
		UserAgent:      "ingate-authz/" + version.String(),
	})
	if err != nil {
		return nil, err
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

// Start 等待首次 Caller 列表同步后持续监听密钥和授权变化。
func (c *CredentialCache) Start(ctx context.Context) error {
	if !c.running.CompareAndSwap(false, true) {
		return errors.New("caller credential cache is already running")
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer close(c.done)
	defer cancel()
	c.lifecycleMu.Lock()
	c.cancel = cancel
	stopping := c.stopping
	c.lifecycleMu.Unlock()
	if stopping {
		cancel()
	}
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

// Stop 停止 Caller 资源监听并等待 informer 退出。
func (c *CredentialCache) Stop(ctx context.Context) error {
	c.lifecycleMu.Lock()
	c.stopping = true
	cancel := c.cancel
	c.lifecycleMu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop Caller credential cache: %w", ctx.Err())
	}
}

// Ready 表示首次 Caller 列表已经同步完成。
func (c *CredentialCache) Ready() bool {
	return c.ready.Load()
}

// Lookup 按公开访问密钥 ID 查询当前授权凭据。
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
	ambiguousKeyIDs := make(map[string]bool)
	for _, caller := range callers {
		if !caller.Spec.Enabled {
			continue
		}
		if !validCredentialConfiguration(caller) {
			c.logger.Error("ignore invalid Caller credential configuration", "caller_id", caller.Name)
			continue
		}
		for _, key := range caller.Spec.AccessKeys {
			if !key.Enabled {
				continue
			}
			if ambiguousKeyIDs[key.ID] {
				continue
			}
			if _, duplicate := credentials[key.ID]; duplicate {
				// 声明式 API 允许直接写资源；若外部调用方错误复用了密钥 ID，
				// 必须让该 ID 整体失效，不能依赖 informer 列表顺序选择某个 Caller
				delete(credentials, key.ID)
				ambiguousKeyIDs[key.ID] = true
				continue
			}
			var expiresAt time.Time
			if key.ExpiresAt != nil {
				expiresAt = key.ExpiresAt.Time
			}
			credential := biz.NewCredential(
				caller.Name,
				key.SecretDigest,
				caller.Spec.RouteRefs,
				expiresAt,
			)
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

func validCredentialConfiguration(caller *gatewayv1.Caller) bool {
	if !resourceconfig.IsCanonicalID(caller.Name) ||
		len(caller.Spec.RouteRefs) > callerconfig.MaxRouteRefs ||
		len(caller.Spec.AccessKeys) > callerconfig.MaxAccessKeys {
		return false
	}

	routeIDs := make(map[string]bool, len(caller.Spec.RouteRefs))
	for _, routeID := range caller.Spec.RouteRefs {
		if !resourceconfig.IsCanonicalID(routeID) || routeIDs[routeID] {
			return false
		}
		routeIDs[routeID] = true
	}

	keyIDs := make(map[string]bool, len(caller.Spec.AccessKeys))
	for _, key := range caller.Spec.AccessKeys {
		if !resourceconfig.IsCanonicalID(key.ID) || keyIDs[key.ID] ||
			!accesskey.IsValidDigest(key.SecretDigest) || key.CreatedAt.IsZero() ||
			(key.ExpiresAt != nil && !key.ExpiresAt.After(key.CreatedAt.Time)) {
			return false
		}
		keyIDs[key.ID] = true
	}
	return true
}

// Package apiserver 从声明式 API 同步 AI ExtProc 执行请求所需的配置。
package apiserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"k8s.io/client-go/tools/cache"

	"github.com/lgc202/ingate/internal/aiextproc/biz/tokenquota"
	"github.com/lgc202/ingate/internal/aiextproc/conf"
	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/apiserverclient"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate/internal/pkg/generated/informers/externalversions"
	gatewaylisters "github.com/lgc202/ingate/internal/pkg/generated/listers/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/upstreamconfig"
	"github.com/lgc202/ingate/internal/pkg/version"
)

// ConfigCache 通过 informer 保持 AI 请求执行配置的本地只读副本。
// 配置事件与请求读取可以并发执行。
type ConfigCache struct {
	logger            *slog.Logger
	factory           informers.SharedInformerFactory
	upstreams         gatewaylisters.UpstreamLister
	tokenQuotaHandler cache.ResourceEventHandlerRegistration

	tokenQuotaMu       sync.RWMutex
	tokenQuotaPolicies map[string]compiledTokenQuotaPolicy
	policiesByCaller   map[string]map[string]tokenquota.Policy

	ready   atomic.Bool
	running atomic.Bool
	done    chan struct{}

	lifecycleMu sync.Mutex
	cancel      context.CancelFunc
	stopping    bool
}

// NewConfigCache 创建 AI 请求执行配置缓存。
func NewConfigCache(
	apiServer *conf.Data_APIServer,
	logger *slog.Logger,
) (*ConfigCache, error) {
	restConfig, err := apiserverclient.NewConfig(apiserverclient.Options{
		MasterURL:      apiServer.GetMaster(),
		KubeconfigPath: apiServer.GetKubeconfig(),
		BearerToken:    apiServer.GetBearerToken(),
		UserAgent:      "ingate-ai-extproc/" + version.String(),
	})
	if err != nil {
		return nil, err
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create API Server resource client: %w", err)
	}

	// resync 为零只关闭周期性 Update 重放；informer 仍会执行初始 List、持续 Watch，
	// 并在连接中断或 resourceVersion 失效后自动恢复。
	factory := informers.NewSharedInformerFactory(client, 0)
	gatewayInformers := factory.Gateway().V1()
	quotaInformer := gatewayInformers.TokenQuotaPolicies()
	configCache := &ConfigCache{
		logger:             logger,
		factory:            factory,
		upstreams:          gatewayInformers.Upstreams().Lister(),
		tokenQuotaPolicies: make(map[string]compiledTokenQuotaPolicy),
		policiesByCaller:   make(map[string]map[string]tokenquota.Policy),
		done:               make(chan struct{}),
	}
	quotaHandler, err := quotaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: configCache.upsertTokenQuotaPolicy,
		UpdateFunc: func(_, current any) {
			configCache.upsertTokenQuotaPolicy(current)
		},
		DeleteFunc: configCache.deleteTokenQuotaPolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("register TokenQuotaPolicy event handler: %w", err)
	}
	configCache.tokenQuotaHandler = quotaHandler
	return configCache, nil
}

// Start 同步首次列表后持续监听 AI 请求执行配置。
func (c *ConfigCache) Start(ctx context.Context) error {
	if !c.running.CompareAndSwap(false, true) {
		return errors.New("AI execution config cache is already running")
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
		return fmt.Errorf("sync AI execution config cache for %v", resourceType)
	}
	if !cache.WaitForCacheSync(runCtx.Done(), c.tokenQuotaHandler.HasSynced) {
		if runCtx.Err() != nil {
			return nil
		}
		return errors.New("sync TokenQuotaPolicy event handler")
	}
	c.ready.Store(true)
	<-runCtx.Done()
	return nil
}

// Stop 停止资源监听并等待 informer 退出。
func (c *ConfigCache) Stop(ctx context.Context) error {
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
		return fmt.Errorf("stop AI execution config cache: %w", ctx.Err())
	}
}

// Ready 表示首次 AI 执行配置列表及额度索引已同步完成。
func (c *ConfigCache) Ready() bool {
	return c.ready.Load()
}

// APIKey 在当前模型 Service 协议与 xDS 选路结果一致时返回访问密钥。
func (c *ConfigCache) APIKey(
	serviceID string,
	expectedProtocol aiprotocol.UpstreamProtocol,
) (string, error) {
	if !c.ready.Load() {
		return "", errors.New("AI execution config cache is not ready")
	}
	upstream, err := c.upstreams.Get(serviceID)
	if err != nil {
		return "", fmt.Errorf("get model service %q: %w", serviceID, err)
	}
	if upstream.Spec.Model == nil {
		return "", fmt.Errorf("service %q is not a model service", serviceID)
	}
	actualProtocol, valid := modelServiceProtocol(upstream.Spec.Model.Protocol)
	if !valid {
		return "", fmt.Errorf("model service %q has an invalid protocol", serviceID)
	}
	if actualProtocol != expectedProtocol {
		return "", fmt.Errorf("model service %q protocol changed while xDS was converging", serviceID)
	}
	if !upstreamconfig.IsValidModelAPIKey(upstream.Spec.Model.APIKey) {
		return "", fmt.Errorf("model service %q has an invalid API key", serviceID)
	}
	return upstream.Spec.Model.APIKey, nil
}

func modelServiceProtocol(protocol resource.ModelProtocol) (aiprotocol.UpstreamProtocol, bool) {
	switch protocol {
	case resource.ModelProtocolOpenAI:
		return aiprotocol.UpstreamProtocolOpenAI, true
	case resource.ModelProtocolAnthropic:
		return aiprotocol.UpstreamProtocolAnthropic, true
	default:
		return "", false
	}
}

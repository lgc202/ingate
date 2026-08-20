// Package modelservice 同步 AI ExtProc 执行请求所需的模型服务配置
package modelservice

import (
	"context"
	"fmt"
	"sync/atomic"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/aiextproc/conf"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate/pkg/generated/informers/externalversions"
	gatewaylisters "github.com/lgc202/ingate/pkg/generated/listers/gateway/v1"
)

// Credentials 是 AI ExtProc 转发模型请求时注入的服务凭据
// APIKey 只存在于 AI ExtProc 内存，不会进入 Envoy xDS 或访问日志
type Credentials struct {
	APIKey string
}

// Cache 通过 Upstream informer 保持模型服务配置的本地只读副本
type Cache struct {
	factory informers.SharedInformerFactory
	lister  gatewaylisters.UpstreamLister

	ready   atomic.Bool
	started chan struct{}
	done    chan struct{}
	cancel  context.CancelFunc
}

// NewCache 创建模型服务配置缓存
func NewCache(apiServer *conf.Data_APIServer) (*Cache, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags(apiServer.GetMaster(), apiServer.GetKubeconfig())
	if err != nil {
		return nil, fmt.Errorf("build API Server client config: %w", err)
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create API Server resource client: %w", err)
	}

	// resync 为零只关闭周期性 Update 重放；informer 仍会执行初始 List、持续 Watch，
	// 并在连接中断或 resourceVersion 失效后自动恢复，不需要把该实现参数暴露为进程配置
	factory := informers.NewSharedInformerFactory(client, 0)
	upstreams := factory.Gateway().V1().Upstreams()
	return &Cache{
		factory: factory,
		lister:  upstreams.Lister(),
		started: make(chan struct{}),
		done:    make(chan struct{}),
	}, nil
}

// Start 同步首次列表后持续监听模型服务配置
func (c *Cache) Start(ctx context.Context) error {
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
		return fmt.Errorf("sync model service cache for %v", resourceType)
	}
	c.ready.Store(true)
	<-runCtx.Done()
	return nil
}

// Stop 停止资源监听并等待 informer 退出
func (c *Cache) Stop(ctx context.Context) error {
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

// Ready 表示首次模型服务列表已同步完成
func (c *Cache) Ready() bool {
	return c.ready.Load()
}

// Get 返回指定模型服务当前生效的凭据
func (c *Cache) Get(serviceID string) (Credentials, error) {
	upstream, err := c.lister.Get(serviceID)
	if err != nil {
		return Credentials{}, fmt.Errorf("get model service %q: %w", serviceID, err)
	}
	if upstream.Spec.Model == nil {
		return Credentials{}, fmt.Errorf("service %q is not a model service", serviceID)
	}
	return Credentials{
		APIKey: upstream.Spec.Model.APIKey,
	}, nil
}

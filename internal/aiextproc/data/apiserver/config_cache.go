// Package apiserver 从声明式 API 同步 AI ExtProc 执行请求所需的配置
package apiserver

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync/atomic"
	"time"
	_ "time/tzdata"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/aiextproc/biz/tokenquota"
	"github.com/lgc202/ingate/internal/aiextproc/conf"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate/internal/pkg/generated/informers/externalversions"
	gatewaylisters "github.com/lgc202/ingate/internal/pkg/generated/listers/gateway/v1"
)

// ConfigCache 通过 informer 保持模型服务和 Token 额度策略的本地只读副本
type ConfigCache struct {
	factory            informers.SharedInformerFactory
	upstreams          gatewaylisters.UpstreamLister
	tokenQuotaPolicies gatewaylisters.TokenQuotaPolicyLister

	ready   atomic.Bool
	started chan struct{}
	done    chan struct{}
	cancel  context.CancelFunc
}

// NewConfigCache 创建 AI 请求执行配置缓存
func NewConfigCache(apiServer *conf.Data_APIServer) (*ConfigCache, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags(apiServer.GetMaster(), apiServer.GetKubeconfig())
	if err != nil {
		return nil, fmt.Errorf("build API Server client config: %w", err)
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create API Server resource client: %w", err)
	}

	// resync 为零只关闭周期性 Update 重放；informer 仍会执行初始 List、持续 Watch，
	// 并在连接中断或 resourceVersion 失效后自动恢复
	factory := informers.NewSharedInformerFactory(client, 0)
	gatewayInformers := factory.Gateway().V1()
	return &ConfigCache{
		factory:            factory,
		upstreams:          gatewayInformers.Upstreams().Lister(),
		tokenQuotaPolicies: gatewayInformers.TokenQuotaPolicies().Lister(),
		started:            make(chan struct{}),
		done:               make(chan struct{}),
	}, nil
}

// Start 同步首次列表后持续监听 AI 请求执行配置
func (c *ConfigCache) Start(ctx context.Context) error {
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
		return fmt.Errorf("sync AI execution config cache for %v", resourceType)
	}
	c.ready.Store(true)
	<-runCtx.Done()
	return nil
}

// Stop 停止资源监听并等待 informer 退出
func (c *ConfigCache) Stop(ctx context.Context) error {
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

// Ready 表示首次 AI 执行配置列表已同步完成
func (c *ConfigCache) Ready() bool {
	return c.ready.Load()
}

// APIKey 返回指定模型 Service 当前生效的访问密钥
func (c *ConfigCache) APIKey(serviceID string) (string, error) {
	upstream, err := c.upstreams.Get(serviceID)
	if err != nil {
		return "", fmt.Errorf("get model service %q: %w", serviceID, err)
	}
	if upstream.Spec.Model == nil {
		return "", fmt.Errorf("service %q is not a model service", serviceID)
	}
	return upstream.Spec.Model.APIKey, nil
}

// ActivePolicies 返回当前调用方命中的全部已启用 Token 额度策略
func (c *ConfigCache) ActivePolicies(callerID string) ([]tokenquota.Policy, error) {
	resources, err := c.tokenQuotaPolicies.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list token quota policies: %w", err)
	}
	policies := make([]tokenquota.Policy, 0)
	for _, policy := range resources {
		if !policy.Spec.Enabled || !targetsCaller(policy.Spec.TargetRefs, callerID) {
			continue
		}
		location, err := time.LoadLocation(policy.Spec.TimeZone)
		if err != nil {
			return nil, fmt.Errorf("load token quota policy %q time zone: %w", policy.Name, err)
		}
		limits := make([]tokenquota.Limit, 0, len(policy.Spec.Limits))
		for _, limit := range policy.Spec.Limits {
			limits = append(limits, tokenquota.Limit{
				Period: tokenQuotaPeriod(limit.Period),
				Tokens: limit.Tokens,
			})
		}
		policies = append(policies, tokenquota.Policy{
			ID:       policy.Name,
			Name:     policy.Spec.DisplayName,
			TimeZone: location,
			Limits:   limits,
		})
	}
	slices.SortFunc(policies, func(a, b tokenquota.Policy) int {
		return cmp.Compare(a.ID, b.ID)
	})
	return policies, nil
}

func targetsCaller(refs []resource.PolicyTargetRef, callerID string) bool {
	return slices.ContainsFunc(refs, func(ref resource.PolicyTargetRef) bool {
		return ref.Kind == resource.KindCaller && ref.Name == callerID
	})
}

func tokenQuotaPeriod(period resource.TokenQuotaPeriod) tokenquota.Period {
	switch period {
	case resource.TokenQuotaPeriodDay:
		return tokenquota.PeriodDay
	case resource.TokenQuotaPeriodWeek:
		return tokenquota.PeriodWeek
	case resource.TokenQuotaPeriodMonth:
		return tokenquota.PeriodMonth
	default:
		return ""
	}
}

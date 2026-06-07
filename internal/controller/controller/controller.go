// Package controller 实现声明式资源的状态收敛主循环
package controller

import (
	"context"
	"log/slog"
	"time"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/lgc202/ingate/internal/core/compiler"
	"github.com/lgc202/ingate/internal/core/pipeline"
	"github.com/lgc202/ingate/internal/core/target/builtin"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate/pkg/generated/informers/externalversions"
	gatewaylisters "github.com/lgc202/ingate/pkg/generated/listers/gateway/v1"
)

type routeIndexName string

const (
	gatewayQueueName                           = "gateway"
	indexTargetSeparator                       = "/"
	routeIndexParentRef         routeIndexName = "parentRef"
	routeIndexUpstreamRef       routeIndexName = "upstreamRef"
	aiRouteIndexParentRef       routeIndexName = "aiParentRef"
	aiRouteIndexProvider        routeIndexName = "aiProviderRef"
	aiRouteIndexModel           routeIndexName = "aiModelRef"
	aiRouteIndexPolicy          routeIndexName = "aiPolicyRef"
	aiModelIndexProvider        routeIndexName = "aiModelProviderRef"
	rateLimitPolicyIndexRedis   routeIndexName = "rateLimitPolicyRedisRef"
	policyBindingIndexTargetRef routeIndexName = "policyBindingTargetRef"
	policyBindingIndexPolicy    routeIndexName = "policyBindingPolicyRef"
	pluginBindingIndexTargetRef routeIndexName = "pluginBindingTargetRef"
	pluginBindingIndexPlugin    routeIndexName = "pluginBindingPluginRef"
)

// Controller 监听声明式资源变化并触发编译
type Controller struct {
	client               clientset.Interface
	factory              informers.SharedInformerFactory
	gatewayLister        gatewaylisters.GatewayLister
	routeLister          gatewaylisters.RouteLister
	upstreamLister       gatewaylisters.UpstreamLister
	aiRouteLister        gatewaylisters.AIRouteLister
	aiProviderLister     gatewaylisters.AIProviderLister
	aiModelLister        gatewaylisters.AIModelLister
	aiPolicyLister       gatewaylisters.AIPolicyLister
	authPolicyLister     gatewaylisters.AuthPolicyLister
	rateLimitLister      gatewaylisters.RateLimitPolicyLister
	redisStoreLister     gatewaylisters.RedisStoreLister
	pluginLister         gatewaylisters.PluginLister
	routeIndexer         cache.Indexer
	aiRouteIndexer       cache.Indexer
	aiModelIndexer       cache.Indexer
	rateLimitIndexer     cache.Indexer
	policyBindingIndexer cache.Indexer
	pluginBindingIndexer cache.Indexer
	pipeline             pipeline.Pipeline
	target               string
	queue                workqueue.TypedRateLimitingInterface[string]
	logger               *slog.Logger
}

// New 创建 controller 实例
func New(client clientset.Interface, target string, resyncPeriod time.Duration, logger *slog.Logger) (*Controller, error) {
	registry, err := builtin.NewRegistry()
	if err != nil {
		return nil, err
	}

	factory := informers.NewSharedInformerFactory(client, resyncPeriod)
	gatewayInformers := factory.Gateway().V1()
	routeInformer := gatewayInformers.Routes().Informer()
	aiRouteInformer := gatewayInformers.AIRoutes().Informer()
	aiModelInformer := gatewayInformers.AIModels().Informer()
	rateLimitInformer := gatewayInformers.RateLimitPolicies().Informer()
	policyBindingInformer := gatewayInformers.PolicyBindings().Informer()
	pluginBindingInformer := gatewayInformers.PluginBindings().Informer()
	if err := routeInformer.AddIndexers(cache.Indexers{
		string(routeIndexParentRef):   routeParentRefIndex,
		string(routeIndexUpstreamRef): routeUpstreamRefIndex,
	}); err != nil {
		return nil, err
	}
	if err := aiRouteInformer.AddIndexers(cache.Indexers{
		string(aiRouteIndexParentRef): aiRouteParentRefIndex,
		string(aiRouteIndexProvider):  aiRouteProviderRefIndex,
		string(aiRouteIndexModel):     aiRouteModelRefIndex,
		string(aiRouteIndexPolicy):    aiRoutePolicyRefIndex,
	}); err != nil {
		return nil, err
	}
	if err := aiModelInformer.AddIndexers(cache.Indexers{
		string(aiModelIndexProvider): aiModelProviderRefIndex,
	}); err != nil {
		return nil, err
	}
	if err := rateLimitInformer.AddIndexers(cache.Indexers{
		string(rateLimitPolicyIndexRedis): rateLimitPolicyRedisRefIndex,
	}); err != nil {
		return nil, err
	}
	if err := policyBindingInformer.AddIndexers(cache.Indexers{
		string(policyBindingIndexTargetRef): policyBindingTargetRefIndex,
		string(policyBindingIndexPolicy):    policyBindingPolicyRefIndex,
	}); err != nil {
		return nil, err
	}
	if err := pluginBindingInformer.AddIndexers(cache.Indexers{
		string(pluginBindingIndexTargetRef): pluginBindingTargetRefIndex,
		string(pluginBindingIndexPlugin):    pluginBindingPluginRefIndex,
	}); err != nil {
		return nil, err
	}

	return &Controller{
		client:               client,
		factory:              factory,
		gatewayLister:        gatewayInformers.Gateways().Lister(),
		routeLister:          gatewayInformers.Routes().Lister(),
		upstreamLister:       gatewayInformers.Upstreams().Lister(),
		aiRouteLister:        gatewayInformers.AIRoutes().Lister(),
		aiProviderLister:     gatewayInformers.AIProviders().Lister(),
		aiModelLister:        gatewayInformers.AIModels().Lister(),
		aiPolicyLister:       gatewayInformers.AIPolicies().Lister(),
		authPolicyLister:     gatewayInformers.AuthPolicies().Lister(),
		rateLimitLister:      gatewayInformers.RateLimitPolicies().Lister(),
		redisStoreLister:     gatewayInformers.RedisStores().Lister(),
		pluginLister:         gatewayInformers.Plugins().Lister(),
		routeIndexer:         routeInformer.GetIndexer(),
		aiRouteIndexer:       aiRouteInformer.GetIndexer(),
		aiModelIndexer:       aiModelInformer.GetIndexer(),
		rateLimitIndexer:     rateLimitInformer.GetIndexer(),
		policyBindingIndexer: policyBindingInformer.GetIndexer(),
		pluginBindingIndexer: pluginBindingInformer.GetIndexer(),
		pipeline: pipeline.Pipeline{
			Compiler: compiler.Compiler{},
			Registry: registry,
		},
		target: target,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: gatewayQueueName},
		),
		logger: logger,
	}, nil
}

// Run 启动 controller 主循环
func (c *Controller) Run(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		c.queue.ShutDown()
		c.factory.Shutdown()
	}()
	go func() {
		<-runCtx.Done()
		c.queue.ShutDown()
	}()

	if err := c.registerEventHandlers(); err != nil {
		return err
	}

	c.factory.Start(runCtx.Done())
	if err := c.waitForCacheSync(runCtx); err != nil {
		return err
	}

	if err := c.enqueueAllGateways(); err != nil {
		return err
	}
	for c.processNextWorkItem() {
		if runCtx.Err() != nil {
			return runCtx.Err()
		}
	}
	return runCtx.Err()
}

func (c *Controller) processNextWorkItem() bool {
	gatewayName, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(gatewayName)

	if err := c.reconcileGateway(gatewayName); err != nil {
		c.queue.AddRateLimited(gatewayName)
		return true
	}
	c.queue.Forget(gatewayName)
	return true
}

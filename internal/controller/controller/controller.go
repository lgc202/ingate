// Package controller 实现声明式资源的状态收敛主循环
package controller

import (
	"context"
	"io"
	"time"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/lgc202/ingate-next/internal/core/compiler"
	"github.com/lgc202/ingate-next/internal/core/pipeline"
	"github.com/lgc202/ingate-next/internal/core/target/builtin"
	clientset "github.com/lgc202/ingate-next/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate-next/pkg/generated/informers/externalversions"
	gatewaylisters "github.com/lgc202/ingate-next/pkg/generated/listers/gateway/v1"
)

type routeIndexName string

const (
	gatewayQueueName                     = "gateway"
	routeIndexParentRef   routeIndexName = "parentRef"
	routeIndexUpstreamRef routeIndexName = "upstreamRef"
)

// Controller 监听声明式资源变化并触发编译
type Controller struct {
	client         clientset.Interface
	factory        informers.SharedInformerFactory
	gatewayLister  gatewaylisters.GatewayLister
	upstreamLister gatewaylisters.UpstreamLister
	routeIndexer   cache.Indexer
	pipeline       pipeline.Pipeline
	target         string
	queue          workqueue.TypedRateLimitingInterface[string]
	stdout         io.Writer
}

// New 创建 controller 实例
func New(client clientset.Interface, target string, resyncPeriod time.Duration, stdout io.Writer) (*Controller, error) {
	registry, err := builtin.NewRegistry()
	if err != nil {
		return nil, err
	}

	factory := informers.NewSharedInformerFactory(client, resyncPeriod)
	gatewayInformers := factory.Gateway().V1()
	routeInformer := gatewayInformers.Routes().Informer()
	if err := routeInformer.AddIndexers(cache.Indexers{
		string(routeIndexParentRef):   routeParentRefIndex,
		string(routeIndexUpstreamRef): routeUpstreamRefIndex,
	}); err != nil {
		return nil, err
	}

	return &Controller{
		client:         client,
		factory:        factory,
		gatewayLister:  gatewayInformers.Gateways().Lister(),
		upstreamLister: gatewayInformers.Upstreams().Lister(),
		routeIndexer:   routeInformer.GetIndexer(),
		pipeline: pipeline.Pipeline{
			Compiler: compiler.Compiler{},
			Registry: registry,
		},
		target: target,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: gatewayQueueName},
		),
		stdout: stdout,
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

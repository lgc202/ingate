// Package controller 实现声明式资源的状态收敛主循环
package controller

import (
	"context"
	"fmt"
	"io"
	"slices"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/lgc202/ingate-next/internal/core/compiler"
	"github.com/lgc202/ingate-next/internal/core/pipeline"
	"github.com/lgc202/ingate-next/internal/core/target/builtin"
	resource "github.com/lgc202/ingate-next/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate-next/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate-next/pkg/generated/informers/externalversions"
	gatewaylisters "github.com/lgc202/ingate-next/pkg/generated/listers/gateway/v1"
)

const gatewayQueueName = "gateway"

// Controller 监听声明式资源变化并触发编译
type Controller struct {
	factory        informers.SharedInformerFactory
	gatewayLister  gatewaylisters.GatewayLister
	routeLister    gatewaylisters.RouteLister
	upstreamLister gatewaylisters.UpstreamLister
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

	return &Controller{
		factory:        factory,
		gatewayLister:  gatewayInformers.Gateways().Lister(),
		routeLister:    gatewayInformers.Routes().Lister(),
		upstreamLister: gatewayInformers.Upstreams().Lister(),
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

func (c *Controller) registerEventHandlers() error {
	gatewayHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			c.enqueueGatewayObject(obj)
		},
		UpdateFunc: func(oldObj, newObj any) {
			c.enqueueGatewayObject(oldObj)
			c.enqueueGatewayObject(newObj)
		},
		DeleteFunc: func(obj any) {
			c.enqueueGatewayObject(obj)
		},
	}
	routeHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			c.enqueueRouteObject(obj)
		},
		UpdateFunc: func(oldObj, newObj any) {
			c.enqueueRouteObject(oldObj)
			c.enqueueRouteObject(newObj)
		},
		DeleteFunc: func(obj any) {
			c.enqueueRouteObject(obj)
		},
	}
	upstreamHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			c.enqueueUpstreamObject(obj)
		},
		UpdateFunc: func(oldObj, newObj any) {
			c.enqueueUpstreamObject(oldObj)
			c.enqueueUpstreamObject(newObj)
		},
		DeleteFunc: func(obj any) {
			c.enqueueUpstreamObject(obj)
		},
	}

	gatewayInformers := c.factory.Gateway().V1()
	if _, err := gatewayInformers.Gateways().Informer().AddEventHandler(gatewayHandler); err != nil {
		return err
	}
	if _, err := gatewayInformers.Routes().Informer().AddEventHandler(routeHandler); err != nil {
		return err
	}
	if _, err := gatewayInformers.Upstreams().Informer().AddEventHandler(upstreamHandler); err != nil {
		return err
	}
	return nil
}

func (c *Controller) waitForCacheSync(ctx context.Context) error {
	for _, synced := range c.factory.WaitForCacheSync(ctx.Done()) {
		if synced {
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("cache sync failed")
	}
	return nil
}

func (c *Controller) enqueueAllGateways() error {
	gateways, err := c.gatewayLister.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, gateway := range gateways {
		c.enqueueGateway(gateway.Name)
	}
	return nil
}

func (c *Controller) enqueueGatewayObject(obj any) {
	gateway, ok := objectAs[*resource.Gateway](obj)
	if !ok {
		return
	}
	c.enqueueGateway(gateway.Name)
}

func (c *Controller) enqueueRouteObject(obj any) {
	route, ok := objectAs[*resource.Route](obj)
	if !ok {
		return
	}
	for _, parentRef := range route.Spec.ParentRefs {
		c.enqueueGateway(parentRef)
	}
}

func (c *Controller) enqueueUpstreamObject(obj any) {
	upstream, ok := objectAs[*resource.Upstream](obj)
	if !ok {
		return
	}

	routes, err := c.routeLister.List(labels.Everything())
	if err != nil {
		return
	}
	for _, route := range routes {
		if routeUsesUpstream(route, upstream.Name) {
			for _, parentRef := range route.Spec.ParentRefs {
				c.enqueueGateway(parentRef)
			}
		}
	}
}

func (c *Controller) enqueueGateway(name string) {
	if name == "" {
		return
	}
	c.queue.Add(name)
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

func (c *Controller) reconcileGateway(gatewayName string) error {
	bundle, found, err := c.bundleForGateway(gatewayName)
	if err != nil {
		return err
	}
	if !found {
		fmt.Fprintf(c.stdout, "skipped target=%s gateway=%s reason=not-found\n", c.target, gatewayName)
		return nil
	}

	snapshot, err := c.pipeline.BuildGatewaySnapshotForTarget(bundle, gatewayName, c.target)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "reconciled target=%s gateway=%s routes=%d upstreams=%d snapshot=%s\n",
		c.target,
		snapshot.Gateway,
		len(bundle.Routes),
		len(bundle.Upstreams),
		snapshot.Version,
	)
	return nil
}

func (c *Controller) bundleForGateway(gatewayName string) (resource.Bundle, bool, error) {
	if _, err := c.gatewayLister.Get(gatewayName); err != nil {
		if apierrors.IsNotFound(err) {
			return resource.Bundle{}, false, nil
		}
		return resource.Bundle{}, false, err
	}

	gateways, err := c.gatewayLister.List(labels.Everything())
	if err != nil {
		return resource.Bundle{}, false, err
	}
	routes, err := c.routeLister.List(labels.Everything())
	if err != nil {
		return resource.Bundle{}, false, err
	}
	upstreams, err := c.upstreamLister.List(labels.Everything())
	if err != nil {
		return resource.Bundle{}, false, err
	}

	bundle := resource.Bundle{
		Gateways: make([]resource.Gateway, 0, len(gateways)),
		Routes:   make([]resource.Route, 0, len(routes)),
	}
	for _, gateway := range gateways {
		bundle.Gateways = append(bundle.Gateways, *gateway)
	}

	usedUpstreams := map[string]bool{}
	for _, route := range routes {
		if !routeAttachedToGateway(route, gatewayName) {
			continue
		}
		bundle.Routes = append(bundle.Routes, *route)
		for _, rule := range route.Spec.Rules {
			for _, upstreamRef := range rule.UpstreamRefs {
				usedUpstreams[upstreamRef.Name] = true
			}
		}
	}

	bundle.Upstreams = make([]resource.Upstream, 0, len(usedUpstreams))
	for _, upstream := range upstreams {
		if !usedUpstreams[upstream.Name] {
			continue
		}
		bundle.Upstreams = append(bundle.Upstreams, *upstream)
	}
	return bundle, true, nil
}

func objectAs[T any](obj any) (T, bool) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	value, ok := obj.(T)
	return value, ok
}

func routeAttachedToGateway(route *resource.Route, gatewayName string) bool {
	return slices.Contains(route.Spec.ParentRefs, gatewayName)
}

func routeUsesUpstream(route *resource.Route, upstreamName string) bool {
	for _, rule := range route.Spec.Rules {
		for _, upstreamRef := range rule.UpstreamRefs {
			if upstreamRef.Name == upstreamName {
				return true
			}
		}
	}
	return false
}

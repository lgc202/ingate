// Package controller 实现声明式资源的状态收敛主循环
package controller

import (
	"context"
	"fmt"
	"io"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	"github.com/lgc202/ingate-next/internal/core/compiler"
	"github.com/lgc202/ingate-next/internal/core/pipeline"
	"github.com/lgc202/ingate-next/internal/core/target/builtin"
	resource "github.com/lgc202/ingate-next/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate-next/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate-next/pkg/generated/informers/externalversions"
	gatewaylisters "github.com/lgc202/ingate-next/pkg/generated/listers/gateway/v1"
)

const triggerBufferSize = 1

// Controller 监听声明式资源变化并触发编译
type Controller struct {
	factory        informers.SharedInformerFactory
	gatewayLister  gatewaylisters.GatewayLister
	routeLister    gatewaylisters.RouteLister
	upstreamLister gatewaylisters.UpstreamLister
	pipeline       pipeline.Pipeline
	target         string
	trigger        chan struct{}
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
		target:  target,
		trigger: make(chan struct{}, triggerBufferSize),
		stdout:  stdout,
	}, nil
}

// Run 启动 controller 主循环
func (c *Controller) Run(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		c.factory.Shutdown()
	}()

	if err := c.registerEventHandlers(); err != nil {
		return err
	}

	c.factory.Start(runCtx.Done())
	if err := c.waitForCacheSync(runCtx); err != nil {
		return err
	}

	c.enqueue()
	for {
		select {
		case <-runCtx.Done():
			return runCtx.Err()
		case <-c.trigger:
			if err := c.reconcile(); err != nil {
				return err
			}
		}
	}
}

func (c *Controller) registerEventHandlers() error {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			c.enqueue()
		},
		UpdateFunc: func(oldObj, newObj any) {
			c.enqueue()
		},
		DeleteFunc: func(obj any) {
			c.enqueue()
		},
	}

	gatewayInformers := c.factory.Gateway().V1()
	if _, err := gatewayInformers.Gateways().Informer().AddEventHandler(handler); err != nil {
		return err
	}
	if _, err := gatewayInformers.Routes().Informer().AddEventHandler(handler); err != nil {
		return err
	}
	if _, err := gatewayInformers.Upstreams().Informer().AddEventHandler(handler); err != nil {
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

func (c *Controller) enqueue() {
	select {
	case c.trigger <- struct{}{}:
	default:
	}
}

func (c *Controller) reconcile() error {
	bundle, err := c.bundle()
	if err != nil {
		return err
	}

	snapshots, err := c.pipeline.BuildGatewaySnapshotsForTarget(bundle, c.target)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "reconciled target=%s gateways=%d routes=%d upstreams=%d snapshots=%d\n",
		c.target,
		len(bundle.Gateways),
		len(bundle.Routes),
		len(bundle.Upstreams),
		len(snapshots),
	)
	return nil
}

func (c *Controller) bundle() (resource.Bundle, error) {
	gateways, err := c.gatewayLister.List(labels.Everything())
	if err != nil {
		return resource.Bundle{}, err
	}
	routes, err := c.routeLister.List(labels.Everything())
	if err != nil {
		return resource.Bundle{}, err
	}
	upstreams, err := c.upstreamLister.List(labels.Everything())
	if err != nil {
		return resource.Bundle{}, err
	}

	bundle := resource.Bundle{
		Gateways:  make([]resource.Gateway, 0, len(gateways)),
		Routes:    make([]resource.Route, 0, len(routes)),
		Upstreams: make([]resource.Upstream, 0, len(upstreams)),
	}
	for _, gateway := range gateways {
		bundle.Gateways = append(bundle.Gateways, *gateway)
	}
	for _, route := range routes {
		bundle.Routes = append(bundle.Routes, *route)
	}
	for _, upstream := range upstreams {
		bundle.Upstreams = append(bundle.Upstreams, *upstream)
	}
	return bundle, nil
}

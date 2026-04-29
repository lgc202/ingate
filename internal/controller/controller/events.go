package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	resource "github.com/lgc202/ingate-next/pkg/apis/gateway/v1"
)

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

	routes, err := c.routesByIndex(routeIndexUpstreamRef, upstream.Name)
	if err != nil {
		return
	}
	for _, route := range routes {
		for _, parentRef := range route.Spec.ParentRefs {
			c.enqueueGateway(parentRef)
		}
	}
}

func (c *Controller) enqueueGateway(name string) {
	if name == "" {
		return
	}
	c.queue.Add(name)
}

func (c *Controller) routesByIndex(index routeIndexName, value string) ([]*resource.Route, error) {
	items, err := c.routeIndexer.ByIndex(string(index), value)
	if err != nil {
		return nil, err
	}

	routes := make([]*resource.Route, 0, len(items))
	for _, item := range items {
		route, ok := item.(*resource.Route)
		if !ok {
			continue
		}
		routes = append(routes, route)
	}
	return routes, nil
}

func objectAs[T any](obj any) (T, bool) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	value, ok := obj.(T)
	return value, ok
}

func routeParentRefIndex(obj any) ([]string, error) {
	route, ok := obj.(*resource.Route)
	if !ok {
		return nil, nil
	}
	return uniqueStrings(route.Spec.ParentRefs), nil
}

func routeUpstreamRefIndex(obj any) ([]string, error) {
	route, ok := obj.(*resource.Route)
	if !ok {
		return nil, nil
	}

	upstreamNames := make([]string, 0)
	for _, rule := range route.Spec.Rules {
		for _, upstreamRef := range rule.UpstreamRefs {
			upstreamNames = append(upstreamNames, upstreamRef.Name)
		}
	}
	return uniqueStrings(upstreamNames), nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

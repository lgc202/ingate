// Package compiler 将声明式资源编译成运行时无关的 IR
package compiler

import (
	"fmt"
	"slices"

	"github.com/lgc202/ingate-next/internal/core/ir"
	"github.com/lgc202/ingate-next/internal/core/resource"
)

// Compiler 负责把声明式资源编译成逻辑网关模型
type Compiler struct{}

// CompileGateway 从内存资源集合中编译指定 Gateway
func (Compiler) CompileGateway(bundle resource.Bundle, gatewayName string) (ir.LogicalGateway, error) {
	c := gatewayCompiler{
		bundle:          bundle,
		gatewayName:     gatewayName,
		gatewaysByName:  make(map[string]resource.Gateway, len(bundle.Gateways)),
		routesByName:    make(map[string]bool, len(bundle.Routes)),
		upstreamsByName: make(map[string]resource.Upstream, len(bundle.Upstreams)),
	}

	return c.compile()
}

type gatewayCompiler struct {
	bundle      resource.Bundle
	gatewayName string
	gateway     resource.Gateway

	gatewaysByName  map[string]resource.Gateway
	routesByName    map[string]bool
	upstreamsByName map[string]resource.Upstream
}

func (c *gatewayCompiler) compile() (ir.LogicalGateway, error) {
	if err := c.indexGateways(); err != nil {
		return ir.LogicalGateway{}, err
	}
	if err := c.indexRoutes(); err != nil {
		return ir.LogicalGateway{}, err
	}
	if err := c.indexUpstreams(); err != nil {
		return ir.LogicalGateway{}, err
	}

	routes, upstreamOrder, err := c.buildAttachedRoutes()
	if err != nil {
		return ir.LogicalGateway{}, err
	}

	return ir.LogicalGateway{
		Name:      c.gateway.Metadata.Name,
		Listeners: c.buildListeners(),
		Routes:    routes,
		Upstreams: c.buildUsedUpstreams(upstreamOrder),
	}, nil
}

func (c *gatewayCompiler) indexGateways() error {
	for _, gateway := range c.bundle.Gateways {
		if _, ok := c.gatewaysByName[gateway.Metadata.Name]; ok {
			return fmt.Errorf("duplicate gateway %q", gateway.Metadata.Name)
		}
		c.gatewaysByName[gateway.Metadata.Name] = gateway
	}

	gateway, ok := c.gatewaysByName[c.gatewayName]
	if !ok {
		return fmt.Errorf("gateway %q not found", c.gatewayName)
	}
	c.gateway = gateway

	return nil
}

func (c *gatewayCompiler) indexRoutes() error {
	for _, route := range c.bundle.Routes {
		if c.routesByName[route.Metadata.Name] {
			return fmt.Errorf("duplicate route %q", route.Metadata.Name)
		}
		c.routesByName[route.Metadata.Name] = true
		for _, parentRef := range route.Spec.ParentRefs {
			if _, ok := c.gatewaysByName[parentRef]; !ok {
				return fmt.Errorf("route %q references gateway %q", route.Metadata.Name, parentRef)
			}
		}
	}

	return nil
}

func (c *gatewayCompiler) indexUpstreams() error {
	for _, upstream := range c.bundle.Upstreams {
		if _, ok := c.upstreamsByName[upstream.Metadata.Name]; ok {
			return fmt.Errorf("duplicate upstream %q", upstream.Metadata.Name)
		}
		c.upstreamsByName[upstream.Metadata.Name] = upstream
	}

	return nil
}

func (c *gatewayCompiler) buildListeners() []ir.LogicalListener {
	listeners := make([]ir.LogicalListener, 0, len(c.gateway.Spec.Listeners))
	for _, listener := range c.gateway.Spec.Listeners {
		listeners = append(listeners, ir.LogicalListener{
			Name:     listener.Name,
			Protocol: listener.Protocol,
			Port:     listener.Port,
			Hostname: listener.Hostname,
		})
	}
	return listeners
}

func (c *gatewayCompiler) buildAttachedRoutes() ([]ir.LogicalRoute, []string, error) {
	routes := make([]ir.LogicalRoute, 0, len(c.bundle.Routes))
	usedUpstreams := make(map[string]bool)
	var upstreamOrder []string

	for _, route := range c.bundle.Routes {
		if !slices.Contains(route.Spec.ParentRefs, c.gatewayName) {
			continue
		}

		logicalRoute := ir.LogicalRoute{
			Name:      route.Metadata.Name,
			Hostnames: slices.Clone(route.Spec.Hostnames),
			Rules:     make([]ir.LogicalRouteRule, 0, len(route.Spec.Rules)),
		}
		for _, rule := range route.Spec.Rules {
			logicalRule := ir.LogicalRouteRule{
				PathPrefix: rule.PathPrefix,
				Upstreams:  make([]ir.LogicalUpstreamRef, 0, len(rule.UpstreamRefs)),
			}
			for _, upstreamRef := range rule.UpstreamRefs {
				if _, ok := c.upstreamsByName[upstreamRef.Name]; !ok {
					return nil, nil, fmt.Errorf("route %q references upstream %q", route.Metadata.Name, upstreamRef.Name)
				}
				logicalRule.Upstreams = append(logicalRule.Upstreams, ir.LogicalUpstreamRef{
					Name:   upstreamRef.Name,
					Weight: upstreamRef.Weight,
				})
				if !usedUpstreams[upstreamRef.Name] {
					usedUpstreams[upstreamRef.Name] = true
					upstreamOrder = append(upstreamOrder, upstreamRef.Name)
				}
			}
			logicalRoute.Rules = append(logicalRoute.Rules, logicalRule)
		}
		routes = append(routes, logicalRoute)
	}

	return routes, upstreamOrder, nil
}

func (c *gatewayCompiler) buildUsedUpstreams(upstreamOrder []string) []ir.LogicalUpstream {
	upstreams := make([]ir.LogicalUpstream, 0, len(upstreamOrder))
	for _, name := range upstreamOrder {
		upstream := c.upstreamsByName[name]
		logicalUpstream := ir.LogicalUpstream{
			Name:      upstream.Metadata.Name,
			Endpoints: make([]ir.LogicalEndpoint, 0, len(upstream.Spec.Endpoints)),
		}
		for _, endpoint := range upstream.Spec.Endpoints {
			logicalUpstream.Endpoints = append(logicalUpstream.Endpoints, ir.LogicalEndpoint{
				Address: endpoint.Address,
				Port:    endpoint.Port,
			})
		}
		upstreams = append(upstreams, logicalUpstream)
	}

	return upstreams
}

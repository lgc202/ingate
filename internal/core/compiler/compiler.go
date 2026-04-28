// Package compiler turns declared resources into runtime-neutral IR.
package compiler

import (
	"fmt"

	"github.com/lgc202/ingate-next/internal/core/ir"
	"github.com/lgc202/ingate-next/internal/core/resource"
)

// Compiler compiles declared resources into logical gateway intent.
type Compiler struct{}

// CompileGateway compiles one gateway from an in-memory resource bundle.
func (Compiler) CompileGateway(bundle resource.Bundle, gatewayName string) (ir.LogicalGateway, error) {
	var gateway resource.Gateway
	foundGateway := false
	for _, item := range bundle.Gateways {
		if item.Metadata.Name == gatewayName {
			gateway = item
			foundGateway = true
			break
		}
	}
	if !foundGateway {
		return ir.LogicalGateway{}, fmt.Errorf("gateway %q not found", gatewayName)
	}

	upstreamsByName := make(map[string]resource.Upstream, len(bundle.Upstreams))
	for _, upstream := range bundle.Upstreams {
		upstreamsByName[upstream.Metadata.Name] = upstream
	}

	logical := ir.LogicalGateway{
		Name:      gateway.Metadata.Name,
		Listeners: make([]ir.LogicalListener, 0, len(gateway.Spec.Listeners)),
	}
	for _, listener := range gateway.Spec.Listeners {
		logical.Listeners = append(logical.Listeners, ir.LogicalListener{
			Name:     listener.Name,
			Protocol: listener.Protocol,
			Port:     listener.Port,
			Hostname: listener.Hostname,
		})
	}

	usedUpstreams := make(map[string]bool)
	var upstreamOrder []string
	for _, route := range bundle.Routes {
		attached := false
		for _, parentRef := range route.Spec.ParentRefs {
			if parentRef == gatewayName {
				attached = true
				break
			}
		}
		if !attached {
			continue
		}

		logicalRoute := ir.LogicalRoute{
			Name:      route.Metadata.Name,
			Hostnames: append([]string(nil), route.Spec.Hostnames...),
			Rules:     make([]ir.LogicalRouteRule, 0, len(route.Spec.Rules)),
		}
		for _, rule := range route.Spec.Rules {
			logicalRule := ir.LogicalRouteRule{
				PathPrefix: rule.PathPrefix,
				Upstreams:  make([]ir.LogicalUpstreamRef, 0, len(rule.UpstreamRefs)),
			}
			for _, upstreamRef := range rule.UpstreamRefs {
				if _, ok := upstreamsByName[upstreamRef.Name]; !ok {
					return ir.LogicalGateway{}, fmt.Errorf("route %q references upstream %q", route.Metadata.Name, upstreamRef.Name)
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
		logical.Routes = append(logical.Routes, logicalRoute)
	}

	logical.Upstreams = make([]ir.LogicalUpstream, 0, len(upstreamOrder))
	for _, name := range upstreamOrder {
		upstream := upstreamsByName[name]
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
		logical.Upstreams = append(logical.Upstreams, logicalUpstream)
	}

	return logical, nil
}

package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func (c *Controller) reconcileGateway(gatewayName string) error {
	bundle, found, err := c.bundleForGateway(gatewayName)
	if err != nil {
		return err
	}
	if !found {
		if err := c.deleteRuntimeSnapshot(context.Background(), c.target, gatewayName); err != nil {
			return err
		}
		fmt.Fprintf(c.stdout, "deleted target=%s gateway=%s reason=gateway-not-found\n", c.target, gatewayName)
		return nil
	}

	snapshot, err := c.pipeline.BuildGatewaySnapshotForTarget(bundle, gatewayName, c.target)
	if err != nil {
		return err
	}
	if err := c.upsertRuntimeSnapshot(context.Background(), snapshot); err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "reconciled target=%s gateway=%s routes=%d aiRoutes=%d upstreams=%d aiProviders=%d snapshot=%s\n",
		c.target,
		snapshot.Gateway,
		len(bundle.Routes),
		len(bundle.AIRoutes),
		len(bundle.Upstreams),
		len(bundle.AIProviders),
		snapshot.Version,
	)
	return nil
}

func (c *Controller) bundleForGateway(gatewayName string) (resource.Bundle, bool, error) {
	gateway, err := c.gatewayLister.Get(gatewayName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return resource.Bundle{}, false, nil
		}
		return resource.Bundle{}, false, err
	}

	routes, err := c.routesByIndex(routeIndexParentRef, gatewayName)
	if err != nil {
		return resource.Bundle{}, false, err
	}
	aiRoutes, err := c.aiRoutesByIndex(aiRouteIndexParentRef, gatewayName)
	if err != nil {
		return resource.Bundle{}, false, err
	}

	bundle := resource.Bundle{
		Gateways: []resource.Gateway{*gateway},
		Routes:   make([]resource.Route, 0, len(routes)),
		AIRoutes: make([]resource.AIRoute, 0, len(aiRoutes)),
	}

	usedUpstreams := map[string]bool{}
	for _, route := range routes {
		bundle.Routes = append(bundle.Routes, *route)
		for _, rule := range route.Spec.Rules {
			for _, upstreamRef := range rule.UpstreamRefs {
				usedUpstreams[upstreamRef.Name] = true
			}
		}
	}
	usedAIProviders := map[string]bool{}
	for _, route := range aiRoutes {
		bundle.AIRoutes = append(bundle.AIRoutes, *route)
		for _, providerRef := range route.Spec.ProviderRefs {
			usedAIProviders[providerRef.Name] = true
		}
	}

	bundle.Upstreams = make([]resource.Upstream, 0, len(usedUpstreams))
	for upstreamName := range usedUpstreams {
		upstream, err := c.upstreamLister.Get(upstreamName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return resource.Bundle{}, false, err
		}
		bundle.Upstreams = append(bundle.Upstreams, *upstream)
	}
	bundle.AIProviders = make([]resource.AIProvider, 0, len(usedAIProviders))
	for providerName := range usedAIProviders {
		provider, err := c.aiProviderLister.Get(providerName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return resource.Bundle{}, false, err
		}
		bundle.AIProviders = append(bundle.AIProviders, *provider)
	}
	return bundle, true, nil
}

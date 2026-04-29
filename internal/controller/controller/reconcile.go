package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	resource "github.com/lgc202/ingate-next/pkg/apis/gateway/v1"
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

	bundle := resource.Bundle{
		Gateways: []resource.Gateway{*gateway},
		Routes:   make([]resource.Route, 0, len(routes)),
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
	return bundle, true, nil
}

package controller

import (
	"context"
	"maps"
	"slices"

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
		c.logger.Info("runtime snapshot deleted",
			"target", c.target,
			"gateway", gatewayName,
			"reason", "gateway_not_found",
		)
		return nil
	}
	if !bundle.Gateways[0].Spec.Enabled {
		if err := c.deleteRuntimeSnapshot(context.Background(), c.target, gatewayName); err != nil {
			return err
		}
		c.logger.Info("runtime snapshot deleted",
			"target", c.target,
			"gateway", gatewayName,
			"reason", "gateway_disabled",
		)
		return nil
	}

	snapshot, err := c.pipeline.BuildGatewaySnapshotForTarget(bundle, gatewayName, c.target)
	if err != nil {
		return err
	}
	if err := c.upsertRuntimeSnapshot(context.Background(), snapshot); err != nil {
		return err
	}
	c.logger.Info("gateway reconciled",
		"target", c.target,
		"gateway", snapshot.Gateway,
		"routes", len(bundle.Routes),
		"upstreams", len(bundle.Upstreams),
		"snapshot", snapshot.Version,
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

	usedUpstreams := c.appendRoutes(&bundle, routes)
	if err := c.appendUpstreams(&bundle, usedUpstreams); err != nil {
		return resource.Bundle{}, false, err
	}
	if err := c.appendBindingResources(&bundle, gateway.Name); err != nil {
		return resource.Bundle{}, false, err
	}
	return bundle, true, nil
}

func (c *Controller) appendRoutes(bundle *resource.Bundle, routes []*resource.Route) map[string]bool {
	usedUpstreams := map[string]bool{}
	for _, route := range routes {
		if !route.Spec.Enabled {
			continue
		}
		bundle.Routes = append(bundle.Routes, *route)
		for _, rule := range route.Spec.Rules {
			for _, upstreamRef := range rule.UpstreamRefs {
				usedUpstreams[upstreamRef.Name] = true
			}
		}
	}
	return usedUpstreams
}

func (c *Controller) appendUpstreams(bundle *resource.Bundle, names map[string]bool) error {
	for _, upstreamName := range slices.Sorted(maps.Keys(names)) {
		upstream, err := c.upstreamLister.Get(upstreamName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		bundle.Upstreams = append(bundle.Upstreams, *upstream)
	}
	return nil
}

func (c *Controller) appendBindingResources(bundle *resource.Bundle, gatewayName string) error {
	usedRateLimitPolicies := map[string]bool{}
	usedAccessControlPolicies := map[string]bool{}
	if err := c.appendPolicyBindingsForTarget(bundle, usedRateLimitPolicies, usedAccessControlPolicies, resource.KindGateway, gatewayName); err != nil {
		return err
	}
	for _, route := range bundle.Routes {
		if err := c.appendPolicyBindingsForTarget(bundle, usedRateLimitPolicies, usedAccessControlPolicies, resource.KindRoute, route.Name); err != nil {
			return err
		}
	}

	if err := c.appendRateLimitPolicies(bundle, usedRateLimitPolicies); err != nil {
		return err
	}
	if err := c.appendAccessControlPolicies(bundle, usedAccessControlPolicies); err != nil {
		return err
	}
	return c.appendRedisStoresForRateLimitPolicies(bundle)
}

func (c *Controller) appendPolicyBindingsForTarget(bundle *resource.Bundle, usedRateLimitPolicies, usedAccessControlPolicies map[string]bool, kind resource.Kind, name string) error {
	// PolicyBinding 通过 Gateway/Route 生效
	// 这里补进 bundle 后，compiler 会按绑定关系生成内置治理插件的逻辑配置
	bindings, err := c.policyBindingsByIndex(policyBindingIndexTargetRef, targetIndexValue(kind, name))
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		bundle.PolicyBindings = append(bundle.PolicyBindings, *binding)
		for _, policyRef := range binding.Spec.Policies {
			switch policyRef.Kind {
			case resource.KindRateLimitPolicy:
				usedRateLimitPolicies[policyRef.Name] = true
			case resource.KindAccessControlPolicy:
				usedAccessControlPolicies[policyRef.Name] = true
			}
		}
	}
	return nil
}

func (c *Controller) appendAccessControlPolicies(bundle *resource.Bundle, names map[string]bool) error {
	bundle.AccessControlPolicies = make([]resource.AccessControlPolicy, 0, len(names))
	for _, policyName := range slices.Sorted(maps.Keys(names)) {
		policy, err := c.accessControlLister.Get(policyName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		bundle.AccessControlPolicies = append(bundle.AccessControlPolicies, *policy)
	}
	return nil
}

func (c *Controller) appendRateLimitPolicies(bundle *resource.Bundle, names map[string]bool) error {
	bundle.RateLimitPolicies = make([]resource.RateLimitPolicy, 0, len(names))
	for _, policyName := range slices.Sorted(maps.Keys(names)) {
		policy, err := c.rateLimitLister.Get(policyName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		bundle.RateLimitPolicies = append(bundle.RateLimitPolicies, *policy)
	}
	return nil
}

func (c *Controller) appendRedisStoresForRateLimitPolicies(bundle *resource.Bundle) error {
	usedRedisStores := map[string]bool{}
	for _, policy := range bundle.RateLimitPolicies {
		if policy.Spec.Global == nil || policy.Spec.Global.RedisRef == "" {
			continue
		}
		usedRedisStores[policy.Spec.Global.RedisRef] = true
	}

	bundle.RedisStores = make([]resource.RedisStore, 0, len(usedRedisStores))
	for _, storeName := range slices.Sorted(maps.Keys(usedRedisStores)) {
		store, err := c.redisStoreLister.Get(storeName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		bundle.RedisStores = append(bundle.RedisStores, *store)
	}
	return nil
}

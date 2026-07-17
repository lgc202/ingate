package reconcile

import (
	"cmp"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/lgc202/ingate/internal/envoy/config"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	gatewaylisters "github.com/lgc202/ingate/pkg/generated/listers/gateway/v1"
)

type resourceListers struct {
	gateways              gatewaylisters.GatewayLister
	routes                gatewaylisters.RouteLister
	upstreams             gatewaylisters.UpstreamLister
	rateLimitPolicies     gatewaylisters.RateLimitPolicyLister
	accessControlPolicies gatewaylisters.AccessControlPolicyLister
	policyBindings        gatewaylisters.PolicyBindingLister
}

func (l resourceListers) build() (config.ResourceSet, error) {
	gateways, err := l.gateways.List(labels.Everything())
	if err != nil {
		return config.ResourceSet{}, fmt.Errorf("list Gateways: %w", err)
	}
	routes, err := l.routes.List(labels.Everything())
	if err != nil {
		return config.ResourceSet{}, fmt.Errorf("list Routes: %w", err)
	}
	upstreams, err := l.upstreams.List(labels.Everything())
	if err != nil {
		return config.ResourceSet{}, fmt.Errorf("list Upstreams: %w", err)
	}
	rateLimitPolicies, err := l.rateLimitPolicies.List(labels.Everything())
	if err != nil {
		return config.ResourceSet{}, fmt.Errorf("list RateLimitPolicies: %w", err)
	}
	accessControlPolicies, err := l.accessControlPolicies.List(labels.Everything())
	if err != nil {
		return config.ResourceSet{}, fmt.Errorf("list AccessControlPolicies: %w", err)
	}
	policyBindings, err := l.policyBindings.List(labels.Everything())
	if err != nil {
		return config.ResourceSet{}, fmt.Errorf("list PolicyBindings: %w", err)
	}

	resources := config.ResourceSet{
		Gateways:              make([]*gatewayv1.Gateway, 0, len(gateways)),
		Routes:                make([]*gatewayv1.Route, 0, len(routes)),
		Upstreams:             make([]*gatewayv1.Upstream, 0, len(upstreams)),
		RateLimitPolicies:     make([]*gatewayv1.RateLimitPolicy, 0, len(rateLimitPolicies)),
		AccessControlPolicies: make([]*gatewayv1.AccessControlPolicy, 0, len(accessControlPolicies)),
		PolicyBindings:        make([]*gatewayv1.PolicyBinding, 0, len(policyBindings)),
	}
	for _, resource := range gateways {
		resources.Gateways = append(resources.Gateways, resource.DeepCopy())
	}
	for _, resource := range routes {
		resources.Routes = append(resources.Routes, resource.DeepCopy())
	}
	for _, resource := range upstreams {
		resources.Upstreams = append(resources.Upstreams, resource.DeepCopy())
	}
	for _, resource := range rateLimitPolicies {
		resources.RateLimitPolicies = append(resources.RateLimitPolicies, resource.DeepCopy())
	}
	for _, resource := range accessControlPolicies {
		resources.AccessControlPolicies = append(resources.AccessControlPolicies, resource.DeepCopy())
	}
	for _, resource := range policyBindings {
		resources.PolicyBindings = append(resources.PolicyBindings, resource.DeepCopy())
	}

	slices.SortFunc(resources.Gateways, func(a, b *gatewayv1.Gateway) int {
		return cmp.Compare(a.Name, b.Name)
	})
	slices.SortFunc(resources.Routes, func(a, b *gatewayv1.Route) int {
		return cmp.Compare(a.Name, b.Name)
	})
	slices.SortFunc(resources.Upstreams, func(a, b *gatewayv1.Upstream) int {
		return cmp.Compare(a.Name, b.Name)
	})
	slices.SortFunc(resources.RateLimitPolicies, func(a, b *gatewayv1.RateLimitPolicy) int {
		return cmp.Compare(a.Name, b.Name)
	})
	slices.SortFunc(resources.AccessControlPolicies, func(a, b *gatewayv1.AccessControlPolicy) int {
		return cmp.Compare(a.Name, b.Name)
	})
	slices.SortFunc(resources.PolicyBindings, func(a, b *gatewayv1.PolicyBinding) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return resources, nil
}

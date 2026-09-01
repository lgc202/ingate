package apiserver

import (
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// HeaderTransformationPolicyStore 读写 HeaderTransformationPolicy 声明式资源。
type HeaderTransformationPolicyStore = resourceStore[
	resource.HeaderTransformationPolicy,
	*resource.HeaderTransformationPolicy,
	*resource.HeaderTransformationPolicyList,
	resource.HeaderTransformationPolicySpec,
]

// IPRestrictionPolicyStore 读写 IPRestrictionPolicy 声明式资源。
type IPRestrictionPolicyStore = resourceStore[
	resource.IPRestrictionPolicy,
	*resource.IPRestrictionPolicy,
	*resource.IPRestrictionPolicyList,
	resource.IPRestrictionPolicySpec,
]

// MockResponsePolicyStore 读写 MockResponsePolicy 声明式资源。
type MockResponsePolicyStore = resourceStore[
	resource.MockResponsePolicy,
	*resource.MockResponsePolicy,
	*resource.MockResponsePolicyList,
	resource.MockResponsePolicySpec,
]

// RateLimitPolicyStore 读写 RateLimitPolicy 声明式资源。
type RateLimitPolicyStore = resourceStore[
	resource.RateLimitPolicy,
	*resource.RateLimitPolicy,
	*resource.RateLimitPolicyList,
	resource.RateLimitPolicySpec,
]

// TokenQuotaPolicyStore 读写 TokenQuotaPolicy 声明式资源。
type TokenQuotaPolicyStore = resourceStore[
	resource.TokenQuotaPolicy,
	*resource.TokenQuotaPolicy,
	*resource.TokenQuotaPolicyList,
	resource.TokenQuotaPolicySpec,
]

// NewHeaderTransformationPolicyStore 创建 HeaderTransformationPolicy Store。
func NewHeaderTransformationPolicyStore(client clientset.Interface) *HeaderTransformationPolicyStore {
	resources := client.GatewayV1().HeaderTransformationPolicies()
	return newResourceStore(
		"header transformation policy",
		"header transformation policies",
		resources,
		func(list *resource.HeaderTransformationPolicyList) ([]resource.HeaderTransformationPolicy, string) {
			return list.Items, list.Continue
		},
		func(resourceID string, spec resource.HeaderTransformationPolicySpec) *resource.HeaderTransformationPolicy {
			return newResource(
				resourceID,
				resource.KindHeaderTransformationPolicy,
				&resource.HeaderTransformationPolicy{Spec: spec},
			)
		},
		func(object *resource.HeaderTransformationPolicy, spec resource.HeaderTransformationPolicySpec) {
			object.Spec = spec
		},
	)
}

// NewIPRestrictionPolicyStore 创建 IPRestrictionPolicy Store。
func NewIPRestrictionPolicyStore(client clientset.Interface) *IPRestrictionPolicyStore {
	resources := client.GatewayV1().IPRestrictionPolicies()
	return newResourceStore(
		"IP restriction policy",
		"IP restriction policies",
		resources,
		func(list *resource.IPRestrictionPolicyList) ([]resource.IPRestrictionPolicy, string) {
			return list.Items, list.Continue
		},
		func(resourceID string, spec resource.IPRestrictionPolicySpec) *resource.IPRestrictionPolicy {
			return newResource(
				resourceID,
				resource.KindIPRestrictionPolicy,
				&resource.IPRestrictionPolicy{Spec: spec},
			)
		},
		func(object *resource.IPRestrictionPolicy, spec resource.IPRestrictionPolicySpec) { object.Spec = spec },
	)
}

// NewMockResponsePolicyStore 创建 MockResponsePolicy Store。
func NewMockResponsePolicyStore(client clientset.Interface) *MockResponsePolicyStore {
	resources := client.GatewayV1().MockResponsePolicies()
	return newResourceStore(
		"mock response policy",
		"mock response policies",
		resources,
		func(list *resource.MockResponsePolicyList) ([]resource.MockResponsePolicy, string) {
			return list.Items, list.Continue
		},
		func(resourceID string, spec resource.MockResponsePolicySpec) *resource.MockResponsePolicy {
			return newResource(
				resourceID,
				resource.KindMockResponsePolicy,
				&resource.MockResponsePolicy{Spec: spec},
			)
		},
		func(object *resource.MockResponsePolicy, spec resource.MockResponsePolicySpec) { object.Spec = spec },
	)
}

// NewRateLimitPolicyStore 创建 RateLimitPolicy Store。
func NewRateLimitPolicyStore(client clientset.Interface) *RateLimitPolicyStore {
	resources := client.GatewayV1().RateLimitPolicies()
	return newResourceStore(
		"rate limit policy",
		"rate limit policies",
		resources,
		func(list *resource.RateLimitPolicyList) ([]resource.RateLimitPolicy, string) {
			return list.Items, list.Continue
		},
		func(resourceID string, spec resource.RateLimitPolicySpec) *resource.RateLimitPolicy {
			return newResource(
				resourceID,
				resource.KindRateLimitPolicy,
				&resource.RateLimitPolicy{Spec: spec},
			)
		},
		func(object *resource.RateLimitPolicy, spec resource.RateLimitPolicySpec) { object.Spec = spec },
	)
}

// NewTokenQuotaPolicyStore 创建 TokenQuotaPolicy Store。
func NewTokenQuotaPolicyStore(client clientset.Interface) *TokenQuotaPolicyStore {
	resources := client.GatewayV1().TokenQuotaPolicies()
	return newResourceStore(
		"token quota policy",
		"token quota policies",
		resources,
		func(list *resource.TokenQuotaPolicyList) ([]resource.TokenQuotaPolicy, string) {
			return list.Items, list.Continue
		},
		func(resourceID string, spec resource.TokenQuotaPolicySpec) *resource.TokenQuotaPolicy {
			return newResource(
				resourceID,
				resource.KindTokenQuotaPolicy,
				&resource.TokenQuotaPolicy{Spec: spec},
			)
		},
		func(object *resource.TokenQuotaPolicy, spec resource.TokenQuotaPolicySpec) { object.Spec = spec },
	)
}

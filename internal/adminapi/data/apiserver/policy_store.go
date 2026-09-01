package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// HeaderTransformationPolicyStore 读写 HeaderTransformationPolicy 声明式资源。
type HeaderTransformationPolicyStore struct {
	*resourceStore[resource.HeaderTransformationPolicy, *resource.HeaderTransformationPolicy, resource.HeaderTransformationPolicySpec]
}

// IPRestrictionPolicyStore 读写 IPRestrictionPolicy 声明式资源。
type IPRestrictionPolicyStore struct {
	*resourceStore[resource.IPRestrictionPolicy, *resource.IPRestrictionPolicy, resource.IPRestrictionPolicySpec]
}

// MockResponsePolicyStore 读写 MockResponsePolicy 声明式资源。
type MockResponsePolicyStore struct {
	*resourceStore[resource.MockResponsePolicy, *resource.MockResponsePolicy, resource.MockResponsePolicySpec]
}

// RateLimitPolicyStore 读写 RateLimitPolicy 声明式资源。
type RateLimitPolicyStore struct {
	*resourceStore[resource.RateLimitPolicy, *resource.RateLimitPolicy, resource.RateLimitPolicySpec]
}

// TokenQuotaPolicyStore 读写 TokenQuotaPolicy 声明式资源。
type TokenQuotaPolicyStore struct {
	*resourceStore[resource.TokenQuotaPolicy, *resource.TokenQuotaPolicy, resource.TokenQuotaPolicySpec]
}

// NewHeaderTransformationPolicyStore 创建 HeaderTransformationPolicy Store。
func NewHeaderTransformationPolicyStore(client clientset.Interface) *HeaderTransformationPolicyStore {
	return &HeaderTransformationPolicyStore{resourceStore: newResourceStore(
		"header transformation policy",
		"header transformation policies",
		func() createResourceClient[*resource.HeaderTransformationPolicy] {
			return client.GatewayV1().HeaderTransformationPolicies()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.HeaderTransformationPolicy, string, error) {
			resources := client.GatewayV1().HeaderTransformationPolicies()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.HeaderTransformationPolicySpec) *resource.HeaderTransformationPolicy {
			return &resource.HeaderTransformationPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindHeaderTransformationPolicy),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.HeaderTransformationPolicy, spec resource.HeaderTransformationPolicySpec) {
			object.Spec = spec
		},
	)}
}

// NewIPRestrictionPolicyStore 创建 IPRestrictionPolicy Store。
func NewIPRestrictionPolicyStore(client clientset.Interface) *IPRestrictionPolicyStore {
	return &IPRestrictionPolicyStore{resourceStore: newResourceStore(
		"IP restriction policy",
		"IP restriction policies",
		func() createResourceClient[*resource.IPRestrictionPolicy] {
			return client.GatewayV1().IPRestrictionPolicies()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.IPRestrictionPolicy, string, error) {
			resources := client.GatewayV1().IPRestrictionPolicies()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.IPRestrictionPolicySpec) *resource.IPRestrictionPolicy {
			return &resource.IPRestrictionPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindIPRestrictionPolicy),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.IPRestrictionPolicy, spec resource.IPRestrictionPolicySpec) { object.Spec = spec },
	)}
}

// NewMockResponsePolicyStore 创建 MockResponsePolicy Store。
func NewMockResponsePolicyStore(client clientset.Interface) *MockResponsePolicyStore {
	return &MockResponsePolicyStore{resourceStore: newResourceStore(
		"mock response policy",
		"mock response policies",
		func() createResourceClient[*resource.MockResponsePolicy] {
			return client.GatewayV1().MockResponsePolicies()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.MockResponsePolicy, string, error) {
			resources := client.GatewayV1().MockResponsePolicies()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.MockResponsePolicySpec) *resource.MockResponsePolicy {
			return &resource.MockResponsePolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindMockResponsePolicy),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.MockResponsePolicy, spec resource.MockResponsePolicySpec) { object.Spec = spec },
	)}
}

// NewRateLimitPolicyStore 创建 RateLimitPolicy Store。
func NewRateLimitPolicyStore(client clientset.Interface) *RateLimitPolicyStore {
	return &RateLimitPolicyStore{resourceStore: newResourceStore(
		"rate limit policy",
		"rate limit policies",
		func() createResourceClient[*resource.RateLimitPolicy] {
			return client.GatewayV1().RateLimitPolicies()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.RateLimitPolicy, string, error) {
			resources := client.GatewayV1().RateLimitPolicies()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.RateLimitPolicySpec) *resource.RateLimitPolicy {
			return &resource.RateLimitPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindRateLimitPolicy),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.RateLimitPolicy, spec resource.RateLimitPolicySpec) { object.Spec = spec },
	)}
}

// NewTokenQuotaPolicyStore 创建 TokenQuotaPolicy Store。
func NewTokenQuotaPolicyStore(client clientset.Interface) *TokenQuotaPolicyStore {
	return &TokenQuotaPolicyStore{resourceStore: newResourceStore(
		"token quota policy",
		"token quota policies",
		func() createResourceClient[*resource.TokenQuotaPolicy] {
			return client.GatewayV1().TokenQuotaPolicies()
		},
		func(ctx context.Context, options metav1.ListOptions) ([]resource.TokenQuotaPolicy, string, error) {
			resources := client.GatewayV1().TokenQuotaPolicies()
			list, err := resources.List(ctx, options)
			if err != nil {
				return nil, "", err
			}
			return list.Items, list.Continue, nil
		},
		func(resourceID string, spec resource.TokenQuotaPolicySpec) *resource.TokenQuotaPolicy {
			return &resource.TokenQuotaPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resource.SchemeGroupVersion.String(),
					Kind:       string(resource.KindTokenQuotaPolicy),
				},
				ObjectMeta: metav1.ObjectMeta{Name: resourceID},
				Spec:       spec,
			}
		},
		func(object *resource.TokenQuotaPolicy, spec resource.TokenQuotaPolicySpec) { object.Spec = spec },
	)}
}

// Package ratelimitpolicy 提供 RateLimitPolicy 的 apiserver 存储与校验
package ratelimitpolicy

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

// NewREST 创建 RateLimitPolicy 主资源与 status 子资源存储
func NewREST(
	optsGetter generic.RESTOptionsGetter,
	typer runtime.ObjectTyper,
	guard *apiregistry.DisplayNameGuard,
) (*apiregistry.REST, *apiregistry.StatusREST, error) {
	return apiregistry.NewStorage(optsGetter, apiregistry.StorageDefinition{
		NewObject:        func() runtime.Object { return &resource.RateLimitPolicy{} },
		NewList:          func() runtime.Object { return &resource.RateLimitPolicyList{} },
		Resource:         resource.Resource(resource.ResourceRateLimitPolicies),
		SingularResource: resource.Resource(resource.ResourceRateLimitPolicy),
		Strategy:         newStrategy(typer),
		StatusStrategy:   newStatusStrategy(typer),
		DisplayName:      func(obj runtime.Object) string { return obj.(*resource.RateLimitPolicy).Spec.DisplayName },
	}, guard)
}

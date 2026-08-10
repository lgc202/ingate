// Package iprestrictionpolicy 提供 IPRestrictionPolicy 的 API Server 存储与校验
package iprestrictionpolicy

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

// NewREST 创建 IPRestrictionPolicy 主资源与 status 子资源存储
func NewREST(
	optsGetter generic.RESTOptionsGetter,
	typer runtime.ObjectTyper,
	guard *apiregistry.DisplayNameGuard,
) (*apiregistry.REST, *apiregistry.StatusREST, error) {
	return apiregistry.NewStorage(optsGetter, apiregistry.StorageDefinition{
		NewObject:        func() runtime.Object { return &resource.IPRestrictionPolicy{} },
		NewList:          func() runtime.Object { return &resource.IPRestrictionPolicyList{} },
		Resource:         resource.Resource(resource.ResourceIPRestrictionPolicies),
		SingularResource: resource.Resource(resource.ResourceIPRestrictionPolicy),
		Strategy:         newStrategy(typer),
		StatusStrategy:   newStatusStrategy(typer),
		DisplayName:      func(obj runtime.Object) string { return obj.(*resource.IPRestrictionPolicy).Spec.DisplayName },
	}, guard)
}

// Package headertransformationpolicy 提供 HeaderTransformationPolicy 的 API Server 存储与校验
package headertransformationpolicy

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
)

// NewREST 创建 HeaderTransformationPolicy 主资源与 status 子资源存储
func NewREST(
	optsGetter generic.RESTOptionsGetter,
	typer runtime.ObjectTyper,
) (*genericregistry.Store, *apiregistry.StatusREST, error) {
	return apiregistry.NewStorage(optsGetter, apiregistry.StorageDefinition{
		NewObject:        func() runtime.Object { return &resource.HeaderTransformationPolicy{} },
		NewList:          func() runtime.Object { return &resource.HeaderTransformationPolicyList{} },
		Resource:         resource.Resource(resource.ResourceHeaderTransformationPolicies),
		SingularResource: resource.Resource(resource.ResourceHeaderTransformationPolicy),
		Strategy:         newStrategy(typer),
		StatusStrategy:   newStatusStrategy(typer),
	})
}

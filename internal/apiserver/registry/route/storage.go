// Package route 提供 Route 的 apiserver 存储与校验
package route

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

// NewREST 创建 Route 主资源与 status 子资源存储
func NewREST(
	optsGetter generic.RESTOptionsGetter,
	typer runtime.ObjectTyper,
) (*apiregistry.REST, *apiregistry.StatusREST, error) {
	return apiregistry.NewStorage(optsGetter, apiregistry.StorageDefinition{
		NewObject:        func() runtime.Object { return &resource.Route{} },
		NewList:          func() runtime.Object { return &resource.RouteList{} },
		Resource:         resource.Resource(resource.ResourceRoutes),
		SingularResource: resource.Resource(resource.ResourceRoute),
		Strategy:         newStrategy(typer),
		StatusStrategy:   newStatusStrategy(typer),
	})
}

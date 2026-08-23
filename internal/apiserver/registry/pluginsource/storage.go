// Package pluginsource 提供 PluginSource 的 API Server 存储与校验
package pluginsource

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
)

// NewREST 创建 PluginSource 主资源与 status 子资源存储
func NewREST(
	optsGetter generic.RESTOptionsGetter,
	typer runtime.ObjectTyper,
) (*genericregistry.Store, *apiregistry.StatusREST, error) {
	return apiregistry.NewStorage(optsGetter, apiregistry.StorageDefinition{
		NewObject:        func() runtime.Object { return &resource.PluginSource{} },
		NewList:          func() runtime.Object { return &resource.PluginSourceList{} },
		Resource:         resource.Resource(resource.ResourcePluginSources),
		SingularResource: resource.Resource(resource.ResourcePluginSource),
		Strategy:         newStrategy(typer),
		StatusStrategy:   newStatusStrategy(typer),
	})
}

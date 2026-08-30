// Package upstream 提供 Upstream 的 API Server 存储与校验。
package upstream

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
)

// NewREST 创建 Upstream 主资源与 status 子资源存储。
func NewREST(
	optsGetter generic.RESTOptionsGetter,
	typer runtime.ObjectTyper,
) (*genericregistry.Store, *apiregistry.StatusREST, error) {
	return apiregistry.NewStorage(optsGetter, apiregistry.StorageDefinition{
		NewObject:        func() runtime.Object { return &resource.Upstream{} },
		NewList:          func() runtime.Object { return &resource.UpstreamList{} },
		Resource:         resource.Resource(resource.ResourceUpstreams),
		SingularResource: resource.Resource(resource.ResourceUpstream),
		Strategy:         newStrategy(typer),
		StatusStrategy:   newStatusStrategy(typer),
	})
}

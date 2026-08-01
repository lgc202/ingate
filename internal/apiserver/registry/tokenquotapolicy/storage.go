// Package tokenquotapolicy 提供 TokenQuotaPolicy 的 apiserver 存储与校验
package tokenquotapolicy

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

// NewREST 创建 TokenQuotaPolicy 主资源与 status 子资源存储
func NewREST(
	optsGetter generic.RESTOptionsGetter,
	typer runtime.ObjectTyper,
) (*apiregistry.REST, *apiregistry.StatusREST, error) {
	return apiregistry.NewStorage(optsGetter, apiregistry.StorageDefinition{
		NewObject:        func() runtime.Object { return &resource.TokenQuotaPolicy{} },
		NewList:          func() runtime.Object { return &resource.TokenQuotaPolicyList{} },
		Resource:         resource.Resource(resource.ResourceTokenQuotaPolicies),
		SingularResource: resource.Resource(resource.ResourceTokenQuotaPolicy),
		Strategy:         newStrategy(typer),
		StatusStrategy:   newStatusStrategy(typer),
	})
}

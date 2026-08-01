// Package accesscontrolpolicy 提供 AccessControlPolicy 的 apiserver 存储与校验
package accesscontrolpolicy

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

// NewREST 创建 AccessControlPolicy 主资源与 status 子资源存储
func NewREST(
	optsGetter generic.RESTOptionsGetter,
	typer runtime.ObjectTyper,
) (*apiregistry.REST, *apiregistry.StatusREST, error) {
	return apiregistry.NewStorage(optsGetter, apiregistry.StorageDefinition{
		NewObject:        func() runtime.Object { return &resource.AccessControlPolicy{} },
		NewList:          func() runtime.Object { return &resource.AccessControlPolicyList{} },
		Resource:         resource.Resource(resource.ResourceAccessControlPolicies),
		SingularResource: resource.Resource(resource.ResourceAccessControlPolicy),
		Strategy:         newStrategy(typer),
		StatusStrategy:   newStatusStrategy(typer),
	})
}

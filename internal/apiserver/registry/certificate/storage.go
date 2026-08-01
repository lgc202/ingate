// Package certificate 提供 Certificate 的 apiserver 存储与校验
package certificate

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

// NewREST 创建 Certificate 主资源与 status 子资源存储
func NewREST(
	optsGetter generic.RESTOptionsGetter,
	typer runtime.ObjectTyper,
) (*apiregistry.REST, *apiregistry.StatusREST, error) {
	return apiregistry.NewStorage(optsGetter, apiregistry.StorageDefinition{
		NewObject:        func() runtime.Object { return &resource.Certificate{} },
		NewList:          func() runtime.Object { return &resource.CertificateList{} },
		Resource:         resource.Resource(resource.ResourceCertificates),
		SingularResource: resource.Resource(resource.ResourceCertificate),
		Strategy:         newStrategy(typer),
		StatusStrategy:   newStatusStrategy(typer),
	})
}

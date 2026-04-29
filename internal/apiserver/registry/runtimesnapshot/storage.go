package runtimesnapshot

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"

	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// REST 实现 RuntimeSnapshot 资源的 apiserver RESTStorage
type REST struct {
	*genericregistry.Store
}

// NewREST 创建 RuntimeSnapshot 资源存储
func NewREST(optsGetter generic.RESTOptionsGetter, typer runtime.ObjectTyper) (*REST, error) {
	strategy := newStrategy(typer)
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &gatewayv1.RuntimeSnapshot{} },
		NewListFunc:               func() runtime.Object { return &gatewayv1.RuntimeSnapshotList{} },
		DefaultQualifiedResource:  gatewayv1.Resource(gatewayv1.ResourceRuntimeSnapshots),
		SingularQualifiedResource: gatewayv1.Resource(gatewayv1.ResourceRuntimeSnapshot),

		CreateStrategy:      strategy,
		UpdateStrategy:      strategy,
		DeleteStrategy:      strategy,
		ResetFieldsStrategy: strategy,

		TableConvertor: rest.NewDefaultTableConvertor(gatewayv1.Resource(gatewayv1.ResourceRuntimeSnapshots)),
	}
	if err := store.CompleteWithOptions(&generic.StoreOptions{RESTOptions: optsGetter}); err != nil {
		return nil, err
	}

	return &REST{Store: store}, nil
}

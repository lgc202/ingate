package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"

	commonregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/common"
	backendregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/backend"
	gatewaytable "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/table"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type BackendStorage struct {
	Backend *REST
	Status  *commonregistry.StatusREST
}

type REST struct{ *genericregistry.Store }

var backendCategories = []string{gatewayv1alpha1.CategoryIngate}
var backendShortNames = []string{gatewayv1alpha1.BackendShortName}

func NewStorage(optsGetter generic.RESTOptionsGetter) (BackendStorage, error) {
	backendREST, backendStatusREST, err := NewREST(optsGetter)
	if err != nil {
		return BackendStorage{}, err
	}
	return BackendStorage{Backend: backendREST, Status: backendStatusREST}, nil
}

func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *commonregistry.StatusREST, error) {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &gatewayv1alpha1.Backend{} },
		NewListFunc:               func() runtime.Object { return &gatewayv1alpha1.BackendList{} },
		PredicateFunc:             backendregistry.Matcher,
		DefaultQualifiedResource:  gatewayv1alpha1.Resource(gatewayv1alpha1.BackendResource),
		SingularQualifiedResource: gatewayv1alpha1.Resource(gatewayv1alpha1.BackendSingularResource),
		CreateStrategy:            backendregistry.Strategy,
		UpdateStrategy:            backendregistry.Strategy,
		DeleteStrategy:            backendregistry.Strategy,
		ResetFieldsStrategy:       backendregistry.Strategy,
		TableConvertor:            commonregistry.NewTableConvertor(gatewayv1alpha1.Resource(gatewayv1alpha1.BackendResource), gatewaytable.BackendColumns(), gatewaytable.BackendCells),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: backendregistry.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, nil, err
	}

	statusStore := *store
	statusStore.UpdateStrategy = backendregistry.StatusStrategy
	statusStore.ResetFieldsStrategy = backendregistry.StatusStrategy

	return &REST{Store: store}, commonregistry.NewStatusREST(func() runtime.Object { return &gatewayv1alpha1.Backend{} }, &statusStore), nil
}

func (*REST) Categories() []string { return backendCategories }

func (*REST) ShortNames() []string { return backendShortNames }

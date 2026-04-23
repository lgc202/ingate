package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"

	commonregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/common"
	gatewayregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/gateway"
	gatewaytable "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/table"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type GatewayStorage struct {
	Gateway *REST
	Status  *commonregistry.StatusREST
}

type REST struct{ *genericregistry.Store }

var gatewayCategories = []string{gatewayv1alpha1.CategoryIngate}
var gatewayShortNames = []string{gatewayv1alpha1.GatewayShortName}

func NewStorage(optsGetter generic.RESTOptionsGetter) (GatewayStorage, error) {
	gatewayREST, gatewayStatusREST, err := NewREST(optsGetter)
	if err != nil {
		return GatewayStorage{}, err
	}
	return GatewayStorage{Gateway: gatewayREST, Status: gatewayStatusREST}, nil
}

func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *commonregistry.StatusREST, error) {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &gatewayv1alpha1.Gateway{} },
		NewListFunc:               func() runtime.Object { return &gatewayv1alpha1.GatewayList{} },
		PredicateFunc:             gatewayregistry.Matcher,
		DefaultQualifiedResource:  gatewayv1alpha1.Resource(gatewayv1alpha1.GatewayResource),
		SingularQualifiedResource: gatewayv1alpha1.Resource(gatewayv1alpha1.GatewaySingularResource),
		CreateStrategy:            gatewayregistry.Strategy,
		UpdateStrategy:            gatewayregistry.Strategy,
		DeleteStrategy:            gatewayregistry.Strategy,
		ResetFieldsStrategy:       gatewayregistry.Strategy,
		TableConvertor:            commonregistry.NewTableConvertor(gatewayv1alpha1.Resource(gatewayv1alpha1.GatewayResource), gatewaytable.GatewayColumns(), gatewaytable.GatewayCells),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: gatewayregistry.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, nil, err
	}

	statusStore := *store
	statusStore.UpdateStrategy = gatewayregistry.StatusStrategy
	statusStore.ResetFieldsStrategy = gatewayregistry.StatusStrategy

	return &REST{Store: store}, commonregistry.NewStatusREST(func() runtime.Object { return &gatewayv1alpha1.Gateway{} }, &statusStore), nil
}

func (*REST) Categories() []string { return gatewayCategories }

func (*REST) ShortNames() []string { return gatewayShortNames }

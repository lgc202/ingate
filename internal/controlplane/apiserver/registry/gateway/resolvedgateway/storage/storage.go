package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"

	commonregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/common"
	resolvedgatewayregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/resolvedgateway"
	gatewaytable "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/table"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type ResolvedGatewayStorage struct {
	ResolvedGateway *REST
	Status          *commonregistry.StatusREST
}

type REST struct{ *genericregistry.Store }

var resolvedGatewayCategories = []string{gatewayv1alpha1.CategoryIngate}
var resolvedGatewayShortNames = []string{gatewayv1alpha1.ResolvedGatewayShortName}

func NewStorage(optsGetter generic.RESTOptionsGetter) (ResolvedGatewayStorage, error) {
	resolvedGatewayREST, resolvedGatewayStatusREST, err := NewREST(optsGetter)
	if err != nil {
		return ResolvedGatewayStorage{}, err
	}
	return ResolvedGatewayStorage{ResolvedGateway: resolvedGatewayREST, Status: resolvedGatewayStatusREST}, nil
}

func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *commonregistry.StatusREST, error) {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &gatewayv1alpha1.ResolvedGateway{} },
		NewListFunc:               func() runtime.Object { return &gatewayv1alpha1.ResolvedGatewayList{} },
		PredicateFunc:             resolvedgatewayregistry.Matcher,
		DefaultQualifiedResource:  gatewayv1alpha1.Resource(gatewayv1alpha1.ResolvedGatewayResource),
		SingularQualifiedResource: gatewayv1alpha1.Resource(gatewayv1alpha1.ResolvedGatewaySingularResource),
		CreateStrategy:            resolvedgatewayregistry.Strategy,
		UpdateStrategy:            resolvedgatewayregistry.Strategy,
		DeleteStrategy:            resolvedgatewayregistry.Strategy,
		ResetFieldsStrategy:       resolvedgatewayregistry.Strategy,
		TableConvertor:            commonregistry.NewTableConvertor(gatewayv1alpha1.Resource(gatewayv1alpha1.ResolvedGatewayResource), gatewaytable.ResolvedGatewayColumns(), gatewaytable.ResolvedGatewayCells),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: resolvedgatewayregistry.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, nil, err
	}

	statusStore := *store
	statusStore.UpdateStrategy = resolvedgatewayregistry.StatusStrategy
	statusStore.ResetFieldsStrategy = resolvedgatewayregistry.StatusStrategy

	return &REST{Store: store}, commonregistry.NewStatusREST(func() runtime.Object { return &gatewayv1alpha1.ResolvedGateway{} }, &statusStore), nil
}

func (*REST) Categories() []string { return resolvedGatewayCategories }

func (*REST) ShortNames() []string { return resolvedGatewayShortNames }

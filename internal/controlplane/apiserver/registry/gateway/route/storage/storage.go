package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"

	commonregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/common"
	routeregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/route"
	gatewaytable "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/table"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type RouteStorage struct {
	Route  *REST
	Status *commonregistry.StatusREST
}

type REST struct{ *genericregistry.Store }

var routeCategories = []string{gatewayv1alpha1.CategoryIngate}
var routeShortNames = []string{gatewayv1alpha1.RouteShortName}

func NewStorage(optsGetter generic.RESTOptionsGetter) (RouteStorage, error) {
	routeREST, routeStatusREST, err := NewREST(optsGetter)
	if err != nil {
		return RouteStorage{}, err
	}
	return RouteStorage{Route: routeREST, Status: routeStatusREST}, nil
}

func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *commonregistry.StatusREST, error) {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &gatewayv1alpha1.Route{} },
		NewListFunc:               func() runtime.Object { return &gatewayv1alpha1.RouteList{} },
		PredicateFunc:             routeregistry.Matcher,
		DefaultQualifiedResource:  gatewayv1alpha1.Resource(gatewayv1alpha1.RouteResource),
		SingularQualifiedResource: gatewayv1alpha1.Resource(gatewayv1alpha1.RouteSingularResource),
		CreateStrategy:            routeregistry.Strategy,
		UpdateStrategy:            routeregistry.Strategy,
		DeleteStrategy:            routeregistry.Strategy,
		ResetFieldsStrategy:       routeregistry.Strategy,
		TableConvertor:            commonregistry.NewTableConvertor(gatewayv1alpha1.Resource(gatewayv1alpha1.RouteResource), gatewaytable.RouteColumns(), gatewaytable.RouteCells),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: routeregistry.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, nil, err
	}

	statusStore := *store
	statusStore.UpdateStrategy = routeregistry.StatusStrategy
	statusStore.ResetFieldsStrategy = routeregistry.StatusStrategy

	return &REST{Store: store}, commonregistry.NewStatusREST(func() runtime.Object { return &gatewayv1alpha1.Route{} }, &statusStore), nil
}

func (*REST) Categories() []string { return routeCategories }

func (*REST) ShortNames() []string { return routeShortNames }

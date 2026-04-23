package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"

	commonregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/common"
	secretregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/secret"
	gatewaytable "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/table"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type SecretStorage struct {
	Secret *REST
}

type REST struct{ *genericregistry.Store }

var secretCategories = []string{gatewayv1alpha1.CategoryIngate}
var secretShortNames = []string{gatewayv1alpha1.SecretShortName}

func NewStorage(optsGetter generic.RESTOptionsGetter) (SecretStorage, error) {
	secretREST, err := NewREST(optsGetter)
	if err != nil {
		return SecretStorage{}, err
	}
	return SecretStorage{Secret: secretREST}, nil
}

func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, error) {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &gatewayv1alpha1.Secret{} },
		NewListFunc:               func() runtime.Object { return &gatewayv1alpha1.SecretList{} },
		PredicateFunc:             secretregistry.Matcher,
		DefaultQualifiedResource:  gatewayv1alpha1.Resource(gatewayv1alpha1.SecretResource),
		SingularQualifiedResource: gatewayv1alpha1.Resource(gatewayv1alpha1.SecretSingularResource),
		CreateStrategy:            secretregistry.Strategy,
		UpdateStrategy:            secretregistry.Strategy,
		DeleteStrategy:            secretregistry.Strategy,
		ResetFieldsStrategy:       secretregistry.Strategy,
		TableConvertor:            commonregistry.NewTableConvertor(gatewayv1alpha1.Resource(gatewayv1alpha1.SecretResource), gatewaytable.SecretColumns(), gatewaytable.SecretCells),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: secretregistry.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{Store: store}, nil
}

func (*REST) Categories() []string { return secretCategories }

func (*REST) ShortNames() []string { return secretShortNames }

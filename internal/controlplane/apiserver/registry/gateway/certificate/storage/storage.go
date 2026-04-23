package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"

	commonregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/common"
	certificateregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/certificate"
	gatewaytable "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/table"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type CertificateStorage struct {
	Certificate *REST
	Status      *commonregistry.StatusREST
}

type REST struct{ *genericregistry.Store }

var certificateCategories = []string{gatewayv1alpha1.CategoryIngate}
var certificateShortNames = []string{gatewayv1alpha1.CertificateShortName}

func NewStorage(optsGetter generic.RESTOptionsGetter) (CertificateStorage, error) {
	certificateREST, certificateStatusREST, err := NewREST(optsGetter)
	if err != nil {
		return CertificateStorage{}, err
	}
	return CertificateStorage{Certificate: certificateREST, Status: certificateStatusREST}, nil
}

func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *commonregistry.StatusREST, error) {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &gatewayv1alpha1.Certificate{} },
		NewListFunc:               func() runtime.Object { return &gatewayv1alpha1.CertificateList{} },
		PredicateFunc:             certificateregistry.Matcher,
		DefaultQualifiedResource:  gatewayv1alpha1.Resource(gatewayv1alpha1.CertificateResource),
		SingularQualifiedResource: gatewayv1alpha1.Resource(gatewayv1alpha1.CertificateSingularResource),
		CreateStrategy:            certificateregistry.Strategy,
		UpdateStrategy:            certificateregistry.Strategy,
		DeleteStrategy:            certificateregistry.Strategy,
		ResetFieldsStrategy:       certificateregistry.Strategy,
		TableConvertor:            commonregistry.NewTableConvertor(gatewayv1alpha1.Resource(gatewayv1alpha1.CertificateResource), gatewaytable.CertificateColumns(), gatewaytable.CertificateCells),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: certificateregistry.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, nil, err
	}

	statusStore := *store
	statusStore.UpdateStrategy = certificateregistry.StatusStrategy
	statusStore.ResetFieldsStrategy = certificateregistry.StatusStrategy

	return &REST{Store: store}, commonregistry.NewStatusREST(func() runtime.Object { return &gatewayv1alpha1.Certificate{} }, &statusStore), nil
}

func (*REST) Categories() []string { return certificateCategories }

func (*REST) ShortNames() []string { return certificateShortNames }

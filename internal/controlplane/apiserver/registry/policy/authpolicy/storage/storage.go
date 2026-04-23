package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"

	commonregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/common"
	authpolicyregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/policy/authpolicy"
	policytable "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/policy/table"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
)

type AuthPolicyStorage struct {
	AuthPolicy *REST
	Status     *commonregistry.StatusREST
}

type REST struct{ *genericregistry.Store }

var authPolicyCategories = []string{policyv1alpha1.CategoryIngate}
var authPolicyShortNames = []string{policyv1alpha1.AuthPolicyShortName}

func NewStorage(optsGetter generic.RESTOptionsGetter) (AuthPolicyStorage, error) {
	authPolicyREST, authPolicyStatusREST, err := NewREST(optsGetter)
	if err != nil {
		return AuthPolicyStorage{}, err
	}
	return AuthPolicyStorage{AuthPolicy: authPolicyREST, Status: authPolicyStatusREST}, nil
}

func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *commonregistry.StatusREST, error) {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &policyv1alpha1.AuthPolicy{} },
		NewListFunc:               func() runtime.Object { return &policyv1alpha1.AuthPolicyList{} },
		PredicateFunc:             authpolicyregistry.Matcher,
		DefaultQualifiedResource:  policyv1alpha1.Resource(policyv1alpha1.AuthPolicyResource),
		SingularQualifiedResource: policyv1alpha1.Resource(policyv1alpha1.AuthPolicySingularResource),
		CreateStrategy:            authpolicyregistry.Strategy,
		UpdateStrategy:            authpolicyregistry.Strategy,
		DeleteStrategy:            authpolicyregistry.Strategy,
		ResetFieldsStrategy:       authpolicyregistry.Strategy,
		TableConvertor:            commonregistry.NewTableConvertor(policyv1alpha1.Resource(policyv1alpha1.AuthPolicyResource), policytable.AuthPolicyColumns(), policytable.AuthPolicyCells),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: authpolicyregistry.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, nil, err
	}

	statusStore := *store
	statusStore.UpdateStrategy = authpolicyregistry.StatusStrategy
	statusStore.ResetFieldsStrategy = authpolicyregistry.StatusStrategy

	return &REST{Store: store}, commonregistry.NewStatusREST(func() runtime.Object { return &policyv1alpha1.AuthPolicy{} }, &statusStore), nil
}

func (*REST) Categories() []string { return authPolicyCategories }

func (*REST) ShortNames() []string { return authPolicyShortNames }

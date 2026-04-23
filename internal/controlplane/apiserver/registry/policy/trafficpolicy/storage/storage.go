package storage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"

	commonregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/common"
	policytable "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/policy/table"
	trafficpolicyregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/policy/trafficpolicy"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
)

type TrafficPolicyStorage struct {
	TrafficPolicy *REST
	Status        *commonregistry.StatusREST
}

type REST struct{ *genericregistry.Store }

var trafficPolicyCategories = []string{policyv1alpha1.CategoryIngate}
var trafficPolicyShortNames = []string{policyv1alpha1.TrafficPolicyShortName}

func NewStorage(optsGetter generic.RESTOptionsGetter) (TrafficPolicyStorage, error) {
	trafficPolicyREST, trafficPolicyStatusREST, err := NewREST(optsGetter)
	if err != nil {
		return TrafficPolicyStorage{}, err
	}
	return TrafficPolicyStorage{TrafficPolicy: trafficPolicyREST, Status: trafficPolicyStatusREST}, nil
}

func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *commonregistry.StatusREST, error) {
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &policyv1alpha1.TrafficPolicy{} },
		NewListFunc:               func() runtime.Object { return &policyv1alpha1.TrafficPolicyList{} },
		PredicateFunc:             trafficpolicyregistry.Matcher,
		DefaultQualifiedResource:  policyv1alpha1.Resource(policyv1alpha1.TrafficPolicyResource),
		SingularQualifiedResource: policyv1alpha1.Resource(policyv1alpha1.TrafficPolicySingularResource),
		CreateStrategy:            trafficpolicyregistry.Strategy,
		UpdateStrategy:            trafficpolicyregistry.Strategy,
		DeleteStrategy:            trafficpolicyregistry.Strategy,
		ResetFieldsStrategy:       trafficpolicyregistry.Strategy,
		TableConvertor:            commonregistry.NewTableConvertor(policyv1alpha1.Resource(policyv1alpha1.TrafficPolicyResource), policytable.TrafficPolicyColumns(), policytable.TrafficPolicyCells),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: trafficpolicyregistry.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, nil, err
	}

	statusStore := *store
	statusStore.UpdateStrategy = trafficpolicyregistry.StatusStrategy
	statusStore.ResetFieldsStrategy = trafficpolicyregistry.StatusStrategy

	return &REST{Store: store}, commonregistry.NewStatusREST(func() runtime.Object { return &policyv1alpha1.TrafficPolicy{} }, &statusStore), nil
}

func (*REST) Categories() []string { return trafficPolicyCategories }

func (*REST) ShortNames() []string { return trafficPolicyShortNames }

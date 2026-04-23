package rest

import (
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	serverstorage "k8s.io/apiserver/pkg/server/storage"

	authpolicystorage "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/policy/authpolicy/storage"
	trafficpolicystorage "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/policy/trafficpolicy/storage"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
	ingatescheme "github.com/lgc202/ingate/pkg/apis/scheme"
	apiserverstorage "github.com/lgc202/ingate/pkg/apiserver/storage"
)

// RESTStorageProvider installs the policy api group.
type RESTStorageProvider struct{}

var _ apiserverstorage.RESTStorageProvider = RESTStorageProvider{}

func (p RESTStorageProvider) GroupName() string { return policyv1alpha1.GroupName }

func (p RESTStorageProvider) NewRESTStorage(apiResourceConfigSource serverstorage.APIResourceConfigSource, restOptionsGetter generic.RESTOptionsGetter) (genericapiserver.APIGroupInfo, error) {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(policyv1alpha1.GroupName, ingatescheme.Scheme, ingatescheme.ParameterCodec, ingatescheme.Codecs)

	storageMap, err := p.v1alpha1Storage(apiResourceConfigSource, restOptionsGetter)
	if err != nil {
		return genericapiserver.APIGroupInfo{}, err
	}
	apiGroupInfo.VersionedResourcesStorageMap[policyv1alpha1.SchemeGroupVersion.Version] = storageMap
	return apiGroupInfo, nil
}

func (p RESTStorageProvider) v1alpha1Storage(apiResourceConfigSource serverstorage.APIResourceConfigSource, restOptionsGetter generic.RESTOptionsGetter) (map[string]rest.Storage, error) {
	storageMap := map[string]rest.Storage{}

	if resource := policyv1alpha1.AuthPolicyResource; apiResourceConfigSource.ResourceEnabled(policyv1alpha1.SchemeGroupVersion.WithResource(resource)) {
		authPolicyStorage, err := authpolicystorage.NewStorage(restOptionsGetter)
		if err != nil {
			return storageMap, err
		}
		storageMap[resource] = authPolicyStorage.AuthPolicy
		storageMap[resource+"/"+policyv1alpha1.AuthPolicyStatusSubresource] = authPolicyStorage.Status
	}

	if resource := policyv1alpha1.TrafficPolicyResource; apiResourceConfigSource.ResourceEnabled(policyv1alpha1.SchemeGroupVersion.WithResource(resource)) {
		trafficPolicyStorage, err := trafficpolicystorage.NewStorage(restOptionsGetter)
		if err != nil {
			return storageMap, err
		}
		storageMap[resource] = trafficPolicyStorage.TrafficPolicy
		storageMap[resource+"/"+policyv1alpha1.TrafficPolicyStatusSubresource] = trafficPolicyStorage.Status
	}

	return storageMap, nil
}

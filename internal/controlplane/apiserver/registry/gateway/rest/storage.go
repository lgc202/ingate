package rest

import (
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	serverstorage "k8s.io/apiserver/pkg/server/storage"

	backendstorage "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/backend/storage"
	certificatestorage "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/certificate/storage"
	gatewaystorage "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/gateway/storage"
	routestorage "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/route/storage"
	secretstorage "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/secret/storage"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	ingatescheme "github.com/lgc202/ingate/pkg/apis/scheme"
	apiserverstorage "github.com/lgc202/ingate/pkg/apiserver/storage"
)

// RESTStorageProvider installs the gateway api group.
type RESTStorageProvider struct{}

var _ apiserverstorage.RESTStorageProvider = RESTStorageProvider{}

func (p RESTStorageProvider) GroupName() string { return gatewayv1alpha1.GroupName }

func (p RESTStorageProvider) NewRESTStorage(apiResourceConfigSource serverstorage.APIResourceConfigSource, restOptionsGetter generic.RESTOptionsGetter) (genericapiserver.APIGroupInfo, error) {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(gatewayv1alpha1.GroupName, ingatescheme.Scheme, ingatescheme.ParameterCodec, ingatescheme.Codecs)

	storageMap, err := p.v1alpha1Storage(apiResourceConfigSource, restOptionsGetter)
	if err != nil {
		return genericapiserver.APIGroupInfo{}, err
	}
	apiGroupInfo.VersionedResourcesStorageMap[gatewayv1alpha1.SchemeGroupVersion.Version] = storageMap
	return apiGroupInfo, nil
}

func (p RESTStorageProvider) v1alpha1Storage(apiResourceConfigSource serverstorage.APIResourceConfigSource, restOptionsGetter generic.RESTOptionsGetter) (map[string]rest.Storage, error) {
	storageMap := map[string]rest.Storage{}

	if resource := gatewayv1alpha1.GatewayResource; apiResourceConfigSource.ResourceEnabled(gatewayv1alpha1.SchemeGroupVersion.WithResource(resource)) {
		gatewayStorage, err := gatewaystorage.NewStorage(restOptionsGetter)
		if err != nil {
			return storageMap, err
		}
		storageMap[resource] = gatewayStorage.Gateway
		storageMap[resource+"/"+gatewayv1alpha1.GatewayStatusSubresource] = gatewayStorage.Status
	}

	if resource := gatewayv1alpha1.RouteResource; apiResourceConfigSource.ResourceEnabled(gatewayv1alpha1.SchemeGroupVersion.WithResource(resource)) {
		routeStorage, err := routestorage.NewStorage(restOptionsGetter)
		if err != nil {
			return storageMap, err
		}
		storageMap[resource] = routeStorage.Route
		storageMap[resource+"/"+gatewayv1alpha1.RouteStatusSubresource] = routeStorage.Status
	}

	if resource := gatewayv1alpha1.BackendResource; apiResourceConfigSource.ResourceEnabled(gatewayv1alpha1.SchemeGroupVersion.WithResource(resource)) {
		backendStorage, err := backendstorage.NewStorage(restOptionsGetter)
		if err != nil {
			return storageMap, err
		}
		storageMap[resource] = backendStorage.Backend
		storageMap[resource+"/"+gatewayv1alpha1.BackendStatusSubresource] = backendStorage.Status
	}

	if resource := gatewayv1alpha1.SecretResource; apiResourceConfigSource.ResourceEnabled(gatewayv1alpha1.SchemeGroupVersion.WithResource(resource)) {
		secretStorage, err := secretstorage.NewStorage(restOptionsGetter)
		if err != nil {
			return storageMap, err
		}
		storageMap[resource] = secretStorage.Secret
	}

	if resource := gatewayv1alpha1.CertificateResource; apiResourceConfigSource.ResourceEnabled(gatewayv1alpha1.SchemeGroupVersion.WithResource(resource)) {
		certificateStorage, err := certificatestorage.NewStorage(restOptionsGetter)
		if err != nil {
			return storageMap, err
		}
		storageMap[resource] = certificateStorage.Certificate
		storageMap[resource+"/"+gatewayv1alpha1.CertificateStatusSubresource] = certificateStorage.Status
	}

	return storageMap, nil
}

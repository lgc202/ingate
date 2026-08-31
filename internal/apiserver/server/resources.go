package server

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	callerstorage "github.com/lgc202/ingate/internal/apiserver/registry/caller"
	certificatestorage "github.com/lgc202/ingate/internal/apiserver/registry/certificate"
	gatewaystorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway"
	headertransformationpolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/headertransformationpolicy"
	iprestrictionpolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/iprestrictionpolicy"
	mockresponsepolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/mockresponsepolicy"
	pluginsourcestorage "github.com/lgc202/ingate/internal/apiserver/registry/pluginsource"
	ratelimitpolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/ratelimitpolicy"
	routestorage "github.com/lgc202/ingate/internal/apiserver/registry/route"
	tokenquotapolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/tokenquotapolicy"
	upstreamstorage "github.com/lgc202/ingate/internal/apiserver/registry/upstream"
	wasmpluginstorage "github.com/lgc202/ingate/internal/apiserver/registry/wasmplugin"
	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

type resourceStorageFactory func(
	generic.RESTOptionsGetter,
	runtime.ObjectTyper,
) (*genericregistry.Store, *apiregistry.StatusREST, error)

type resourceRegistration struct {
	resource       gatewayv1.ResourceName
	statusResource gatewayv1.ResourceName
	newStorage     resourceStorageFactory
}

// installResources 把声明式资源及其 status 子资源注册到同一 API Group。
func installResources(
	genericServer *genericapiserver.GenericAPIServer,
	config genericapiserver.CompletedConfig,
) error {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(
		gatewayv1.GroupName,
		Scheme,
		runtime.NewParameterCodec(Scheme),
		Codecs,
	)
	registrations := []resourceRegistration{
		{
			resource:       gatewayv1.ResourceGateways,
			statusResource: gatewayv1.ResourceGatewaysStatus,
			newStorage:     gatewaystorage.NewREST,
		},
		{
			resource:       gatewayv1.ResourceRoutes,
			statusResource: gatewayv1.ResourceRoutesStatus,
			newStorage:     routestorage.NewREST,
		},
		{
			resource:       gatewayv1.ResourceUpstreams,
			statusResource: gatewayv1.ResourceUpstreamsStatus,
			newStorage:     upstreamstorage.NewREST,
		},
		{
			resource:       gatewayv1.ResourceCertificates,
			statusResource: gatewayv1.ResourceCertificatesStatus,
			newStorage:     certificatestorage.NewREST,
		},
		{
			resource:       gatewayv1.ResourceRateLimitPolicies,
			statusResource: gatewayv1.ResourceRateLimitPoliciesStatus,
			newStorage:     ratelimitpolicystorage.NewREST,
		},
		{
			resource:       gatewayv1.ResourceIPRestrictionPolicies,
			statusResource: gatewayv1.ResourceIPRestrictionPoliciesStatus,
			newStorage:     iprestrictionpolicystorage.NewREST,
		},
		{
			resource:       gatewayv1.ResourceCallers,
			statusResource: gatewayv1.ResourceCallersStatus,
			newStorage:     callerstorage.NewREST,
		},
		{
			resource:       gatewayv1.ResourceTokenQuotaPolicies,
			statusResource: gatewayv1.ResourceTokenQuotaPoliciesStatus,
			newStorage:     tokenquotapolicystorage.NewREST,
		},
		{
			resource:       gatewayv1.ResourceWasmPlugins,
			statusResource: gatewayv1.ResourceWasmPluginsStatus,
			newStorage:     wasmpluginstorage.NewREST,
		},
		{
			resource:       gatewayv1.ResourcePluginSources,
			statusResource: gatewayv1.ResourcePluginSourcesStatus,
			newStorage:     pluginsourcestorage.NewREST,
		},
		{
			resource:       gatewayv1.ResourceHeaderTransformationPolicies,
			statusResource: gatewayv1.ResourceHeaderTransformationPoliciesStatus,
			newStorage:     headertransformationpolicystorage.NewREST,
		},
		{
			resource:       gatewayv1.ResourceMockResponsePolicies,
			statusResource: gatewayv1.ResourceMockResponsePoliciesStatus,
			newStorage:     mockresponsepolicystorage.NewREST,
		},
	}
	storage := make(map[string]rest.Storage, len(registrations)*2)
	for _, registration := range registrations {
		resourceStorage, statusStorage, err := registration.newStorage(config.RESTOptionsGetter, Scheme)
		if err != nil {
			return fmt.Errorf("create %s storage: %w", registration.resource, err)
		}
		storage[string(registration.resource)] = resourceStorage
		storage[string(registration.statusResource)] = statusStorage
	}

	apiGroupInfo.VersionedResourcesStorageMap[gatewayv1.SchemeGroupVersion.Version] = storage
	return genericServer.InstallAPIGroup(&apiGroupInfo)
}

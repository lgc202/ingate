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

// installResources 把声明式资源及其 status 子资源注册到同一 API Group
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
		{gatewayv1.ResourceGateways, gatewayv1.ResourceGatewaysStatus, gatewaystorage.NewREST},
		{gatewayv1.ResourceRoutes, gatewayv1.ResourceRoutesStatus, routestorage.NewREST},
		{gatewayv1.ResourceUpstreams, gatewayv1.ResourceUpstreamsStatus, upstreamstorage.NewREST},
		{gatewayv1.ResourceCertificates, gatewayv1.ResourceCertificatesStatus, certificatestorage.NewREST},
		{gatewayv1.ResourceRateLimitPolicies, gatewayv1.ResourceRateLimitPoliciesStatus, ratelimitpolicystorage.NewREST},
		{gatewayv1.ResourceIPRestrictionPolicies, gatewayv1.ResourceIPRestrictionPoliciesStatus, iprestrictionpolicystorage.NewREST},
		{gatewayv1.ResourceCallers, gatewayv1.ResourceCallersStatus, callerstorage.NewREST},
		{gatewayv1.ResourceTokenQuotaPolicies, gatewayv1.ResourceTokenQuotaPoliciesStatus, tokenquotapolicystorage.NewREST},
		{gatewayv1.ResourceWasmPlugins, gatewayv1.ResourceWasmPluginsStatus, wasmpluginstorage.NewREST},
		{gatewayv1.ResourceHeaderTransformationPolicies, gatewayv1.ResourceHeaderTransformationPoliciesStatus, headertransformationpolicystorage.NewREST},
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

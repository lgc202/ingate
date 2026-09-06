package server

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	"github.com/lgc202/ingate/internal/apiserver/registry/caller"
	"github.com/lgc202/ingate/internal/apiserver/registry/certificate"
	"github.com/lgc202/ingate/internal/apiserver/registry/gateway"
	"github.com/lgc202/ingate/internal/apiserver/registry/plugin/source"
	"github.com/lgc202/ingate/internal/apiserver/registry/plugin/wasm"
	"github.com/lgc202/ingate/internal/apiserver/registry/policy/headertransformation"
	"github.com/lgc202/ingate/internal/apiserver/registry/policy/iprestriction"
	"github.com/lgc202/ingate/internal/apiserver/registry/policy/mockresponse"
	"github.com/lgc202/ingate/internal/apiserver/registry/policy/ratelimit"
	"github.com/lgc202/ingate/internal/apiserver/registry/policy/tokenquota"
	"github.com/lgc202/ingate/internal/apiserver/registry/route"
	"github.com/lgc202/ingate/internal/apiserver/registry/upstream"
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
			newStorage:     gateway.NewREST,
		},
		{
			resource:       gatewayv1.ResourceRoutes,
			statusResource: gatewayv1.ResourceRoutesStatus,
			newStorage:     route.NewREST,
		},
		{
			resource:       gatewayv1.ResourceUpstreams,
			statusResource: gatewayv1.ResourceUpstreamsStatus,
			newStorage:     upstream.NewREST,
		},
		{
			resource:       gatewayv1.ResourceCertificates,
			statusResource: gatewayv1.ResourceCertificatesStatus,
			newStorage:     certificate.NewREST,
		},
		{
			resource:       gatewayv1.ResourceRateLimitPolicies,
			statusResource: gatewayv1.ResourceRateLimitPoliciesStatus,
			newStorage:     ratelimit.NewREST,
		},
		{
			resource:       gatewayv1.ResourceIPRestrictionPolicies,
			statusResource: gatewayv1.ResourceIPRestrictionPoliciesStatus,
			newStorage:     iprestriction.NewREST,
		},
		{
			resource:       gatewayv1.ResourceCallers,
			statusResource: gatewayv1.ResourceCallersStatus,
			newStorage:     caller.NewREST,
		},
		{
			resource:       gatewayv1.ResourceTokenQuotaPolicies,
			statusResource: gatewayv1.ResourceTokenQuotaPoliciesStatus,
			newStorage:     tokenquota.NewREST,
		},
		{
			resource:       gatewayv1.ResourceWasmPlugins,
			statusResource: gatewayv1.ResourceWasmPluginsStatus,
			newStorage:     wasm.NewREST,
		},
		{
			resource:       gatewayv1.ResourcePluginSources,
			statusResource: gatewayv1.ResourcePluginSourcesStatus,
			newStorage:     source.NewREST,
		},
		{
			resource:       gatewayv1.ResourceHeaderTransformationPolicies,
			statusResource: gatewayv1.ResourceHeaderTransformationPoliciesStatus,
			newStorage:     headertransformation.NewREST,
		},
		{
			resource:       gatewayv1.ResourceMockResponsePolicies,
			statusResource: gatewayv1.ResourceMockResponsePoliciesStatus,
			newStorage:     mockresponse.NewREST,
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

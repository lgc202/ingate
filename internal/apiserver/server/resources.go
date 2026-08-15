package server

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	certificatestorage "github.com/lgc202/ingate/internal/apiserver/registry/certificate"
	gatewaystorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway"
	iprestrictionpolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/iprestrictionpolicy"
	ratelimitpolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/ratelimitpolicy"
	routestorage "github.com/lgc202/ingate/internal/apiserver/registry/route"
	upstreamstorage "github.com/lgc202/ingate/internal/apiserver/registry/upstream"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

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
	storage := make(map[string]rest.Storage)
	installStatusStorage := func(
		resourceName gatewayv1.ResourceName,
		statusResourceName gatewayv1.ResourceName,
		factory func() (rest.Storage, rest.Storage, error),
	) error {
		resourceStorage, statusStorage, err := factory()
		if err != nil {
			return err
		}
		storage[string(resourceName)] = resourceStorage
		storage[string(statusResourceName)] = statusStorage
		return nil
	}

	if err := installStatusStorage(gatewayv1.ResourceGateways, gatewayv1.ResourceGatewaysStatus, func() (rest.Storage, rest.Storage, error) {
		return gatewaystorage.NewREST(config.RESTOptionsGetter, Scheme)
	}); err != nil {
		return err
	}
	if err := installStatusStorage(gatewayv1.ResourceRoutes, gatewayv1.ResourceRoutesStatus, func() (rest.Storage, rest.Storage, error) {
		return routestorage.NewREST(config.RESTOptionsGetter, Scheme)
	}); err != nil {
		return err
	}
	if err := installStatusStorage(gatewayv1.ResourceUpstreams, gatewayv1.ResourceUpstreamsStatus, func() (rest.Storage, rest.Storage, error) {
		return upstreamstorage.NewREST(config.RESTOptionsGetter, Scheme)
	}); err != nil {
		return err
	}
	if err := installStatusStorage(gatewayv1.ResourceCertificates, gatewayv1.ResourceCertificatesStatus, func() (rest.Storage, rest.Storage, error) {
		return certificatestorage.NewREST(config.RESTOptionsGetter, Scheme)
	}); err != nil {
		return err
	}
	if err := installStatusStorage(gatewayv1.ResourceRateLimitPolicies, gatewayv1.ResourceRateLimitPoliciesStatus, func() (rest.Storage, rest.Storage, error) {
		return ratelimitpolicystorage.NewREST(config.RESTOptionsGetter, Scheme)
	}); err != nil {
		return err
	}
	if err := installStatusStorage(gatewayv1.ResourceIPRestrictionPolicies, gatewayv1.ResourceIPRestrictionPoliciesStatus, func() (rest.Storage, rest.Storage, error) {
		return iprestrictionpolicystorage.NewREST(config.RESTOptionsGetter, Scheme)
	}); err != nil {
		return err
	}

	apiGroupInfo.VersionedResourcesStorageMap[gatewayv1.SchemeGroupVersion.Version] = storage
	return genericServer.InstallAPIGroup(&apiGroupInfo)
}

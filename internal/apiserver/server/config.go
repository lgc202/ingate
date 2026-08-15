package server

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	certificatestorage "github.com/lgc202/ingate/internal/apiserver/registry/certificate"
	gatewaystorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway"
	iprestrictionpolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/iprestrictionpolicy"
	ratelimitpolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/ratelimitpolicy"
	routestorage "github.com/lgc202/ingate/internal/apiserver/registry/route"
	upstreamstorage "github.com/lgc202/ingate/internal/apiserver/registry/upstream"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const serverName = "ingate-apiserver"

// Config 表示 ingate-apiserver 完整配置
type Config struct {
	GenericConfig    *genericapiserver.RecommendedConfig
	DisplayNameGuard *apiregistry.DisplayNameGuard
}

type completedConfig struct {
	GenericConfig    genericapiserver.CompletedConfig
	DisplayNameGuard *apiregistry.DisplayNameGuard
}

// CompletedConfig 表示补全后的 ingate-apiserver 配置
type CompletedConfig struct {
	*completedConfig
}

// Complete 补全 ingate-apiserver 配置
func (c *Config) Complete() CompletedConfig {
	return CompletedConfig{&completedConfig{
		GenericConfig:    c.GenericConfig.Complete(),
		DisplayNameGuard: c.DisplayNameGuard,
	}}
}

// New 创建 ingate-apiserver 实例
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*Server, error) {
	genericServer, err := c.GenericConfig.New(serverName, delegationTarget)
	if err != nil {
		return nil, err
	}

	server := &Server{
		GenericAPIServer: genericServer,
		stop:             make(chan struct{}),
		done:             make(chan struct{}),
	}
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(
		gatewayv1.GroupName,
		Scheme,
		runtime.NewParameterCodec(Scheme),
		Codecs,
	)

	storage := make(map[string]rest.Storage)
	installStatusStorage := func(resourceName, statusResourceName gatewayv1.ResourceName, factory func() (rest.Storage, rest.Storage, error)) error {
		resourceStorage, statusStorage, err := factory()
		if err != nil {
			return err
		}
		storage[string(resourceName)] = resourceStorage
		storage[string(statusResourceName)] = statusStorage
		return nil
	}

	if err := installStatusStorage(gatewayv1.ResourceGateways, gatewayv1.ResourceGatewaysStatus, func() (rest.Storage, rest.Storage, error) {
		return gatewaystorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme, c.DisplayNameGuard)
	}); err != nil {
		return nil, err
	}
	if err := installStatusStorage(gatewayv1.ResourceRoutes, gatewayv1.ResourceRoutesStatus, func() (rest.Storage, rest.Storage, error) {
		return routestorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme, c.DisplayNameGuard)
	}); err != nil {
		return nil, err
	}
	if err := installStatusStorage(gatewayv1.ResourceUpstreams, gatewayv1.ResourceUpstreamsStatus, func() (rest.Storage, rest.Storage, error) {
		return upstreamstorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme, c.DisplayNameGuard)
	}); err != nil {
		return nil, err
	}
	if err := installStatusStorage(gatewayv1.ResourceCertificates, gatewayv1.ResourceCertificatesStatus, func() (rest.Storage, rest.Storage, error) {
		return certificatestorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme, c.DisplayNameGuard)
	}); err != nil {
		return nil, err
	}
	if err := installStatusStorage(gatewayv1.ResourceRateLimitPolicies, gatewayv1.ResourceRateLimitPoliciesStatus, func() (rest.Storage, rest.Storage, error) {
		return ratelimitpolicystorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme, c.DisplayNameGuard)
	}); err != nil {
		return nil, err
	}
	if err := installStatusStorage(gatewayv1.ResourceIPRestrictionPolicies, gatewayv1.ResourceIPRestrictionPoliciesStatus, func() (rest.Storage, rest.Storage, error) {
		return iprestrictionpolicystorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme, c.DisplayNameGuard)
	}); err != nil {
		return nil, err
	}
	apiGroupInfo.VersionedResourcesStorageMap[gatewayv1.SchemeGroupVersion.Version] = storage

	if err := server.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return server, nil
}

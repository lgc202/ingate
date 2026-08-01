package server

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	accesscontrolpolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/accesscontrolpolicy"
	certificatestorage "github.com/lgc202/ingate/internal/apiserver/registry/certificate"
	gatewaystorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway"
	ratelimitpolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/ratelimitpolicy"
	routestorage "github.com/lgc202/ingate/internal/apiserver/registry/route"
	tokenquotapolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/tokenquotapolicy"
	upstreamstorage "github.com/lgc202/ingate/internal/apiserver/registry/upstream"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const serverName = "ingate-apiserver"

// Config 表示 ingate-apiserver 完整配置
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
}

// CompletedConfig 表示补全后的 ingate-apiserver 配置
type CompletedConfig struct {
	*completedConfig
}

// Server 表示 ingate-apiserver 运行实例
type Server struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

// Complete 补全 ingate-apiserver 配置
func (c *Config) Complete() CompletedConfig {
	return CompletedConfig{&completedConfig{
		GenericConfig: c.GenericConfig.Complete(),
	}}
}

// New 创建 ingate-apiserver 实例
func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*Server, error) {
	genericServer, err := c.GenericConfig.New(serverName, delegationTarget)
	if err != nil {
		return nil, err
	}

	server := &Server{GenericAPIServer: genericServer}
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
		return gatewaystorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
	}); err != nil {
		return nil, err
	}
	if err := installStatusStorage(gatewayv1.ResourceRoutes, gatewayv1.ResourceRoutesStatus, func() (rest.Storage, rest.Storage, error) {
		return routestorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
	}); err != nil {
		return nil, err
	}
	if err := installStatusStorage(gatewayv1.ResourceUpstreams, gatewayv1.ResourceUpstreamsStatus, func() (rest.Storage, rest.Storage, error) {
		return upstreamstorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
	}); err != nil {
		return nil, err
	}
	if err := installStatusStorage(gatewayv1.ResourceCertificates, gatewayv1.ResourceCertificatesStatus, func() (rest.Storage, rest.Storage, error) {
		return certificatestorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
	}); err != nil {
		return nil, err
	}
	if err := installStatusStorage(gatewayv1.ResourceRateLimitPolicies, gatewayv1.ResourceRateLimitPoliciesStatus, func() (rest.Storage, rest.Storage, error) {
		return ratelimitpolicystorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
	}); err != nil {
		return nil, err
	}
	if err := installStatusStorage(gatewayv1.ResourceAccessControlPolicies, gatewayv1.ResourceAccessControlPoliciesStatus, func() (rest.Storage, rest.Storage, error) {
		return accesscontrolpolicystorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
	}); err != nil {
		return nil, err
	}
	if err := installStatusStorage(gatewayv1.ResourceTokenQuotaPolicies, gatewayv1.ResourceTokenQuotaPoliciesStatus, func() (rest.Storage, rest.Storage, error) {
		return tokenquotapolicystorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
	}); err != nil {
		return nil, err
	}
	apiGroupInfo.VersionedResourcesStorageMap[gatewayv1.SchemeGroupVersion.Version] = storage

	if err := server.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return server, nil
}

package server

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	accesscontrolpolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/accesscontrolpolicy"
	gatewaystorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway"
	policybindingstorage "github.com/lgc202/ingate/internal/apiserver/registry/policybinding"
	ratelimitpolicystorage "github.com/lgc202/ingate/internal/apiserver/registry/ratelimitpolicy"
	redisstorestorage "github.com/lgc202/ingate/internal/apiserver/registry/redisstore"
	routestorage "github.com/lgc202/ingate/internal/apiserver/registry/route"
	runtimegroupstorage "github.com/lgc202/ingate/internal/apiserver/registry/runtimegroup"
	runtimesnapshotstorage "github.com/lgc202/ingate/internal/apiserver/registry/runtimesnapshot"
	upstreamstorage "github.com/lgc202/ingate/internal/apiserver/registry/upstream"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const serverName = "ingate-apiserver"

// ExtraConfig 表示 ingate-apiserver 自己的扩展配置
type ExtraConfig struct {
	Storage map[string]rest.Storage
}

// Config 表示 ingate-apiserver 完整配置
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
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
		ExtraConfig:   &c.ExtraConfig,
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

	storage := c.ExtraConfig.Storage
	if storage == nil {
		storage = map[string]rest.Storage{}
	}

	// ExtraConfig.Storage 允许测试或上层调用方预置 storage
	// 这里只补齐缺失项，避免覆盖外部传入的自定义实现
	installStorage := func(resourceName gatewayv1.ResourceName, factory func() (rest.Storage, error)) error {
		if _, ok := storage[string(resourceName)]; ok {
			return nil
		}
		resourceStorage, err := factory()
		if err != nil {
			return err
		}
		storage[string(resourceName)] = resourceStorage
		return nil
	}
	installStatusStorage := func(resourceName, statusResourceName gatewayv1.ResourceName, factory func() (rest.Storage, rest.Storage, error)) error {
		_, hasResource := storage[string(resourceName)]
		_, hasStatus := storage[string(statusResourceName)]
		if hasResource && hasStatus {
			return nil
		}
		resourceStorage, statusStorage, err := factory()
		if err != nil {
			return err
		}
		if !hasResource {
			storage[string(resourceName)] = resourceStorage
		}
		if !hasStatus {
			storage[string(statusResourceName)] = statusStorage
		}
		return nil
	}

	if err := installStatusStorage(gatewayv1.ResourceGateways, gatewayv1.ResourceGatewaysStatus, func() (rest.Storage, rest.Storage, error) {
		return gatewaystorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
	}); err != nil {
		return nil, err
	}
	if err := installStatusStorage(gatewayv1.ResourceRuntimeGroups, gatewayv1.ResourceRuntimeGroupsStatus, func() (rest.Storage, rest.Storage, error) {
		return runtimegroupstorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
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
	if err := installStorage(gatewayv1.ResourceRuntimeSnapshots, func() (rest.Storage, error) {
		return runtimesnapshotstorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
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
	if err := installStatusStorage(gatewayv1.ResourceRedisStores, gatewayv1.ResourceRedisStoresStatus, func() (rest.Storage, rest.Storage, error) {
		return redisstorestorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
	}); err != nil {
		return nil, err
	}
	if err := installStatusStorage(gatewayv1.ResourcePolicyBindings, gatewayv1.ResourcePolicyBindingsStatus, func() (rest.Storage, rest.Storage, error) {
		return policybindingstorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
	}); err != nil {
		return nil, err
	}
	apiGroupInfo.VersionedResourcesStorageMap[gatewayv1.SchemeGroupVersion.Version] = storage

	if err := server.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return server, nil
}

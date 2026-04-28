package server

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	gatewaystorage "github.com/lgc202/ingate-next/internal/apiserver/registry/gateway"
	gatewayv1 "github.com/lgc202/ingate-next/pkg/apis/gateway/v1"
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
	if _, ok := storage[string(gatewayv1.ResourceGateways)]; !ok {
		gatewayREST, err := gatewaystorage.NewREST(c.GenericConfig.RESTOptionsGetter, Scheme)
		if err != nil {
			return nil, err
		}
		storage[string(gatewayv1.ResourceGateways)] = gatewayREST
	}
	apiGroupInfo.VersionedResourcesStorageMap[gatewayv1.SchemeGroupVersion.Version] = storage

	if err := server.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return server, nil
}

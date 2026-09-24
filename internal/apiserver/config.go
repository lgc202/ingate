package apiserver

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

const componentName = "ingate-apiserver"

// Config 保存尚可由启动选项修改的 Generic API Server 配置。
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	Scheme        *runtime.Scheme
}

type completedConfig struct {
	genericConfig genericapiserver.CompletedConfig
	scheme        *runtime.Scheme
}

// CompletedConfig 保存已经完成默认值计算的配置。
// 私有指针防止包外绕过 Config.Complete 构造有效值。
type CompletedConfig struct {
	*completedConfig
}

// Server 保存组装完成、尚未运行的 Generic API Server。
type Server struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

// Complete 完成 Generic API Server 配置的默认值计算。
// 它会修改 Config.GenericConfig，应在应用全部启动选项后调用。
func (c *Config) Complete() CompletedConfig {
	completed := &completedConfig{
		genericConfig: c.GenericConfig.Complete(),
		scheme:        c.Scheme,
	}

	return CompletedConfig{completedConfig: completed}
}

// New 组装 Generic API Server 并安装 Gateway API Group，但不启动服务。
func (c completedConfig) New() (*Server, error) {
	genericServer, err := c.genericConfig.New(
		componentName,
		genericapiserver.NewEmptyDelegate(),
	)
	if err != nil {
		return nil, fmt.Errorf("create generic API server: %w", err)
	}

	server := &Server{GenericAPIServer: genericServer}
	if err := installGatewayAPIGroup(
		server.GenericAPIServer,
		c.scheme,
		c.genericConfig.RESTOptionsGetter,
	); err != nil {
		server.GenericAPIServer.Destroy()
		return nil, err
	}

	return server, nil
}

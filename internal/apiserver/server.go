package apiserver

import (
	"fmt"

	genericapiserver "k8s.io/apiserver/pkg/server"
)

// Server 保存组装完成、尚未运行的 Generic API Server。
type Server struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

// New 组装 Generic API Server 并安装核心组和 Gateway API Group，但不启动服务。
func (c completedConfig) New() (*Server, error) {
	genericServer, err := c.genericConfig.New(
		componentName,
		genericapiserver.NewEmptyDelegate(),
	)
	if err != nil {
		return nil, fmt.Errorf("create generic API server: %w", err)
	}

	server := &Server{GenericAPIServer: genericServer}
	if err := installCoreAPIGroup(
		server.GenericAPIServer,
		c.scheme,
		c.genericConfig.RESTOptionsGetter,
	); err != nil {
		server.GenericAPIServer.Destroy()
		return nil, err
	}

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

package apiserver

import (
	"errors"
	"fmt"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"
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
		// Generic Server 尚未创建成功，不能依赖它的 Destroy 回调清理本地工厂。
		if c.namespaceInitializer != nil {
			c.namespaceInitializer.informers.Shutdown()
		}

		return nil, errors.Join(
			fmt.Errorf("create generic API server: %w", err),
			closeSecureServing(c.genericConfig.SecureServing),
		)
	}

	// Run 正常关闭 HTTP Listener；在 API 安装失败、运行失败或重复 Destroy 时，
	// 仍由框架同一个销毁入口回收。原生 Listener 的重复 Close 被视为已完成清理。
	genericServer.RegisterDestroyFunc(func() {
		if err := closeSecureServing(c.genericConfig.SecureServing); err != nil {
			klog.Background().Error(err, "Failed to close API server listener")
		}
	})

	if c.namespaceInitializer != nil {
		// 接管工厂清理：安装失败时 Destroy 立即释放；正常退出时框架先取消 Hook
		// 的 Context，再调用 Destroy，Shutdown 在此等待 Informer 工作协程结束。
		genericServer.RegisterDestroyFunc(c.namespaceInitializer.informers.Shutdown)
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

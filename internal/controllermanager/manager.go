// Package controllermanager 组装共享数据面的 Controller 与 xDS 服务。
package controllermanager

import (
	"fmt"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	gatewaycontroller "github.com/lgc202/ingate/internal/controllermanager/controller/gateway"
	"github.com/lgc202/ingate/internal/controllermanager/dataplane/envoy"
)

// Config 保存 Controller Manager 的 API 连接和运行选项。
type Config struct {
	RESTConfig     *rest.Config
	XDSBindAddress string
	ManagerOptions ctrl.Options
}

// New 创建并配置 Manager，但不启动 Controller 或网络监听。
func (c *Config) New() (ctrl.Manager, error) {
	manager, err := ctrl.NewManager(c.RESTConfig, c.ManagerOptions)
	if err != nil {
		return nil, fmt.Errorf("create controller manager: %w", err)
	}
	if err := manager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("add health check: %w", err)
	}

	publisher := envoy.NewPublisher()
	xdsServer := envoy.NewServer(c.XDSBindAddress, publisher)
	if err := manager.Add(xdsServer); err != nil {
		return nil, fmt.Errorf("add xDS server: %w", err)
	}

	reconciler := gatewaycontroller.New(manager.GetClient(), publisher)
	if err := reconciler.SetupWithManager(manager); err != nil {
		return nil, fmt.Errorf("setup Gateway controller: %w", err)
	}
	if err := manager.AddReadyzCheck("xds", xdsServer.CheckReady); err != nil {
		return nil, fmt.Errorf("add xDS readiness check: %w", err)
	}

	return manager, nil
}

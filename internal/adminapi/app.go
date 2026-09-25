// Package adminapi 组装对外的 HTTP 和 gRPC 管理接口及其依赖。
package adminapi

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v3"
	gatewayhandler "github.com/lgc202/ingate/internal/adminapi/handler/gateway"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	apiserverstore "github.com/lgc202/ingate/internal/adminapi/repository/apiserver"
	"github.com/lgc202/ingate/internal/adminapi/server"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
)

const componentName = "ingate-admin-api"

// Run 组装并运行 Admin API，直到 Context 取消或服务退出。
func Run(ctx context.Context, configFile string) error {
	config, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("load Admin API config: %w", err)
	}

	apiServerStore, err := apiserverstore.New(config.GetApiServer().GetKubeconfig())
	if err != nil {
		return fmt.Errorf("create API server store: %w", err)
	}

	gateways := repository.NewGateway(apiServerStore.Gateways())
	gatewayService := gatewayservice.New(gateways)
	gatewayHandler := gatewayhandler.New(gatewayService)
	handlers := server.Handlers{Gateway: gatewayHandler}
	httpServer, grpcServer := server.New(config.GetServer(), handlers)
	app := kratos.New(
		kratos.Context(ctx),
		kratos.Name(componentName),
		kratos.Server(httpServer, grpcServer),
	)
	if err := app.Run(); err != nil {
		return fmt.Errorf("run Admin API: %w", err)
	}

	return nil
}

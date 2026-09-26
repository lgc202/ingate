// Package adminapi 组装对外的 HTTP 和 gRPC 管理接口及其依赖。
package adminapi

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-kratos/kratos/v3"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	accounthandler "github.com/lgc202/ingate/internal/adminapi/handler/account"
	gatewayhandler "github.com/lgc202/ingate/internal/adminapi/handler/gateway"
	"github.com/lgc202/ingate/internal/adminapi/middleware"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	apiserverstore "github.com/lgc202/ingate/internal/adminapi/repository/apiserver"
	mysqlstore "github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	redisstore "github.com/lgc202/ingate/internal/adminapi/repository/redis"
	"github.com/lgc202/ingate/internal/adminapi/server"
	accountservice "github.com/lgc202/ingate/internal/adminapi/service/account"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
)

const componentName = "ingate-admin-api"

// Run 组装并运行 Admin API，直到 Context 取消或服务退出。
func Run(ctx context.Context, configFile string) error {
	config, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("load Admin API config: %w", err)
	}

	accountStore, err := mysqlstore.Open(config.GetMysql().GetDsn())
	if err != nil {
		return fmt.Errorf("open account store: %w", err)
	}
	defer func() {
		if err := accountStore.Close(); err != nil {
			slog.ErrorContext(ctx, "close account store", "err", err)
		}
	}()

	redisStore, err := redisstore.Open(ctx, config.GetRedis().GetUrl())
	if err != nil {
		return fmt.Errorf("open session store: %w", err)
	}
	defer func() {
		if err := redisStore.Close(); err != nil {
			slog.ErrorContext(ctx, "close session store", "err", err)
		}
	}()

	accounts := repository.NewAccount(accountStore)
	sessions := repository.NewSession(redisStore)
	accountService := accountservice.New(accounts, sessions)
	if err := accountService.Bootstrap(ctx); err != nil {
		return fmt.Errorf("bootstrap initial admin: %w", err)
	}

	apiServerStore, err := apiserverstore.New(config.GetApiServer().GetKubeconfig())
	if err != nil {
		return fmt.Errorf("create API server store: %w", err)
	}

	app := newApp(ctx, config, apiServerStore, accountService)
	if err := app.Run(); err != nil {
		return fmt.Errorf("run Admin API: %w", err)
	}

	return nil
}

func newApp(
	ctx context.Context,
	config *conf.Bootstrap,
	apiServerStore *apiserverstore.Store,
	accountService *accountservice.Service,
) *kratos.App {
	gateways := repository.NewGateway(apiServerStore.Gateways())
	gatewayService := gatewayservice.New(gateways)
	gatewayHandler := gatewayhandler.New(gatewayService)
	accountHandler := accounthandler.New(accountService)

	handlers := server.Handlers{
		Gateway: gatewayHandler,
		Account: accountHandler,
	}
	middlewareChain := server.NewMiddlewareChain(middleware.RequireSession(accountService))
	httpServer, grpcServer := server.New(config.GetServer(), handlers, middlewareChain)

	return kratos.New(
		kratos.Context(ctx),
		kratos.Name(componentName),
		kratos.Server(httpServer, grpcServer),
	)
}

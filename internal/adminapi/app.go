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
	routehandler "github.com/lgc202/ingate/internal/adminapi/handler/route"
	upstreamhandler "github.com/lgc202/ingate/internal/adminapi/handler/upstream"
	"github.com/lgc202/ingate/internal/adminapi/middleware"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	apiserverstore "github.com/lgc202/ingate/internal/adminapi/repository/apiserver"
	mysqlstore "github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	redisstore "github.com/lgc202/ingate/internal/adminapi/repository/redis"
	"github.com/lgc202/ingate/internal/adminapi/server"
	accountservice "github.com/lgc202/ingate/internal/adminapi/service/account"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
)

const componentName = "ingate-admin-api"

// Run 组装并运行 Admin API，直到 Context 取消或服务退出。
func Run(ctx context.Context, configFile string) error {
	config, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("load Admin API config: %w", err)
	}

	mysqlStore, err := mysqlstore.Open(config.GetMysql().GetDsn())
	if err != nil {
		return fmt.Errorf("open MySQL store: %w", err)
	}
	defer func() {
		if err := mysqlStore.Close(); err != nil {
			slog.ErrorContext(ctx, "close MySQL store", "err", err)
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

	accounts := repository.NewAccount(mysqlStore)
	sessions := repository.NewSession(redisStore)
	accountService := accountservice.New(accounts, sessions)
	if err := accountService.Bootstrap(ctx); err != nil {
		return fmt.Errorf("bootstrap initial admin: %w", err)
	}

	apiServerStore, err := apiserverstore.New(config.GetApiServer().GetKubeconfig())
	if err != nil {
		return fmt.Errorf("create API server store: %w", err)
	}

	app := newApp(ctx, config, apiServerStore, mysqlStore, accountService)
	if err := app.Run(); err != nil {
		return fmt.Errorf("run Admin API: %w", err)
	}

	return nil
}

func newApp(
	ctx context.Context,
	config *conf.Bootstrap,
	apiServerStore *apiserverstore.Store,
	mysqlStore *mysqlstore.Store,
	accountService *accountservice.Service,
) *kratos.App {
	gateways := repository.NewGateway(apiServerStore.Gateways(), mysqlStore)
	routes := repository.NewRoute(apiServerStore.Routes(), mysqlStore)
	upstreams := repository.NewUpstream(apiServerStore.Upstreams(), mysqlStore)

	gatewayService := gatewayservice.New(gateways)
	routeService := routeservice.New(routes)
	upstreamService := upstreamservice.New(upstreams)

	gatewayHandler := gatewayhandler.New(gatewayService)
	routeHandler := routehandler.New(routeService)
	upstreamHandler := upstreamhandler.New(upstreamService)
	accountHandler := accounthandler.New(accountService)

	handlers := server.Handlers{
		Gateway:  gatewayHandler,
		Route:    routeHandler,
		Upstream: upstreamHandler,
		Account:  accountHandler,
	}
	middlewareChain := server.NewMiddlewareChain(middleware.RequireSession(accountService))
	httpServer, grpcServer := server.New(config.GetServer(), handlers, middlewareChain)

	return kratos.New(
		kratos.Context(ctx),
		kratos.Name(componentName),
		kratos.Server(httpServer, grpcServer),
	)
}

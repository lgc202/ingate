// Package adminapi 组装对外的 HTTP 和 gRPC 管理接口及其依赖。
package adminapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/go-kratos/kratos/v3"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	accounthandler "github.com/lgc202/ingate/internal/adminapi/handler/account"
	aibackendhandler "github.com/lgc202/ingate/internal/adminapi/handler/aibackend"
	backendhandler "github.com/lgc202/ingate/internal/adminapi/handler/backend"
	gatewayhandler "github.com/lgc202/ingate/internal/adminapi/handler/gateway"
	routehandler "github.com/lgc202/ingate/internal/adminapi/handler/httproute"
	modelroutehandler "github.com/lgc202/ingate/internal/adminapi/handler/modelroute"
	"github.com/lgc202/ingate/internal/adminapi/middleware"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	apiserverstore "github.com/lgc202/ingate/internal/adminapi/repository/apiserver"
	mysqlstore "github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	redisstore "github.com/lgc202/ingate/internal/adminapi/repository/redis"
	"github.com/lgc202/ingate/internal/adminapi/server"
	accountservice "github.com/lgc202/ingate/internal/adminapi/service/account"
	aibackendservice "github.com/lgc202/ingate/internal/adminapi/service/aibackend"
	backendservice "github.com/lgc202/ingate/internal/adminapi/service/backend"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	routeservice "github.com/lgc202/ingate/internal/adminapi/service/httproute"
	modelrouteservice "github.com/lgc202/ingate/internal/adminapi/service/modelroute"
	logconfig "github.com/lgc202/ingate/internal/pkg/log"
	mysqlclient "github.com/lgc202/ingate/internal/pkg/mysql"
	redisclient "github.com/lgc202/ingate/internal/pkg/redis"
	"github.com/lgc202/ingate/internal/pkg/version"
)

const componentName = "ingate-admin-api"

// Run 组装并运行 Admin API，直到 Context 取消或服务退出。
func Run(ctx context.Context, configFile string) (runErr error) {
	config, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("load Admin API config: %w", err)
	}
	logger, closeLog, err := logconfig.New(logConfig(config.GetLog()))
	if err != nil {
		return fmt.Errorf("initialize Admin API log: %w", err)
	}
	defer func() {
		runErr = closeLogger(closeLog, runErr)
	}()
	logger = logger.With("service", componentName)

	db, err := mysqlclient.Open(config.GetMysql().GetDsn())
	if err != nil {
		return fmt.Errorf("open MySQL store: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get MySQL connection pool: %w", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			logger.ErrorContext(ctx, "close MySQL store", "err", err)
		}
	}()
	mysqlStore := mysqlstore.New(db)

	redisClient, err := redisclient.New(redisConfig(config.GetRedis()))
	if err != nil {
		return fmt.Errorf("open session store: %w", err)
	}
	if err := redisClient.Ping(ctx).Err(); err != nil {
		// 清理失败不覆盖原始连接错误，保持启动探测的返回语义。
		_ = redisClient.Close()
		return fmt.Errorf("open session store: ping Redis: %w", err)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.ErrorContext(ctx, "close session store", "err", err)
		}
	}()
	redisStore := redisstore.New(redisClient)

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

	app := newApp(ctx, config, apiServerStore, mysqlStore, accountService, logger)
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
	logger *slog.Logger,
) *kratos.App {
	gateways := repository.NewGateway(apiServerStore.Gateways(), mysqlStore)
	routes := repository.NewHTTPRoute(apiServerStore.HTTPRoutes(), mysqlStore)
	backends := repository.NewBackend(apiServerStore.Backends(), mysqlStore)
	aiBackends := repository.NewAIBackend(apiServerStore.AIBackends(), mysqlStore)
	modelRoutes := repository.NewModelRoute(apiServerStore.ModelRoutes(), mysqlStore)

	gatewayService := gatewayservice.New(gateways)
	routeService := routeservice.New(routes)
	backendService := backendservice.New(backends)
	aiBackendService := aibackendservice.New(aiBackends)
	modelRouteService := modelrouteservice.New(modelRoutes)

	gatewayHandler := gatewayhandler.New(gatewayService)
	routeHandler := routehandler.New(routeService)
	backendHandler := backendhandler.New(backendService)
	aiBackendHandler := aibackendhandler.New(aiBackendService)
	modelRouteHandler := modelroutehandler.New(modelRouteService)
	accountHandler := accounthandler.New(accountService)

	handlers := server.Handlers{
		Gateway:    gatewayHandler,
		HTTPRoute:  routeHandler,
		Backend:    backendHandler,
		AIBackend:  aiBackendHandler,
		ModelRoute: modelRouteHandler,
		Account:    accountHandler,
	}
	middlewareChain := server.NewMiddlewareChain(middleware.RequireSession(accountService))
	httpServer, grpcServer := server.New(config.GetServer(), handlers, middlewareChain)

	return kratos.New(
		kratos.Context(ctx),
		kratos.Name(componentName),
		kratos.Version(version.Get().String()),
		kratos.Logger(logger),
		kratos.Server(httpServer, grpcServer),
	)
}

func closeLogger(closeLog func() error, runErr error) error {
	if err := closeLog(); err != nil {
		return errors.Join(runErr, fmt.Errorf("close Admin API log: %w", err))
	}
	return runErr
}

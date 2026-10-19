// Package adminapi 组装对外的 HTTP 和 gRPC 管理接口及其依赖。
package adminapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-kratos/kratos/v3"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	"github.com/lgc202/ingate/internal/adminapi/handler"
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
	httprouteservice "github.com/lgc202/ingate/internal/adminapi/service/httproute"
	mcprouteservice "github.com/lgc202/ingate/internal/adminapi/service/mcproute"
	modelrouteservice "github.com/lgc202/ingate/internal/adminapi/service/modelroute"
	logconfig "github.com/lgc202/ingate/internal/pkg/log"
	mysqlclient "github.com/lgc202/ingate/internal/pkg/mysql"
	redisclient "github.com/lgc202/ingate/internal/pkg/redis"
	"github.com/lgc202/ingate/internal/pkg/version"
)

const (
	componentName = "ingate-admin-api"
	// shutdownTimeout 限制 HTTP/gRPC 排空等待，避免退出被长请求无限拖住。
	shutdownTimeout = 10 * time.Second
)

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
		if err := closeLog(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close Admin API log: %w", err))
		}
	}()
	logger = logger.With("service", componentName)

	// kubeconfig 的本地错误必须先于建库连接和初始化管理员被发现。
	apiServerStore, err := apiserverstore.New(config.GetApiServer().GetKubeconfig())
	if err != nil {
		return fmt.Errorf("create API server store: %w", err)
	}

	db, err := mysqlclient.Open(mysqlConfig(config.GetMysql()))
	if err != nil {
		return fmt.Errorf("open MySQL store: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get MySQL connection pool: %w", err)
	}

	defer func() {
		if err := sqlDB.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close MySQL store: %w", err))
		}
	}()

	// 初始化不访问数据库；启动前可用性由组件以独立期限确认。
	mysqlPingCtx, cancelMySQLPing := context.WithTimeout(ctx, config.GetMysql().GetPingTimeout().AsDuration())
	defer cancelMySQLPing()
	if err := sqlDB.PingContext(mysqlPingCtx); err != nil {
		return fmt.Errorf("verify MySQL connection: %w", err)
	}

	mysqlStore := mysqlstore.New(db)

	redisClient, err := redisclient.New(redisConfig(config.GetRedis()))
	if err != nil {
		return fmt.Errorf("open session store: %w", err)
	}

	defer func() {
		if err := redisClient.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close session store: %w", err))
		}
	}()

	redisPingCtx, cancelRedisPing := context.WithTimeout(ctx, config.GetRedis().GetPingTimeout().AsDuration())
	defer cancelRedisPing()
	if err := redisClient.Ping(redisPingCtx).Err(); err != nil {
		return fmt.Errorf("open session store: ping Redis: %w", err)
	}

	redisStore := redisstore.New(redisClient)

	accounts := repository.NewAccount(mysqlStore)
	sessions := repository.NewSession(redisStore)
	accountService := accountservice.New(accounts, sessions)
	if err := accountService.Bootstrap(ctx); err != nil {
		return fmt.Errorf("bootstrap initial admin: %w", err)
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
	gateways := repository.NewGateway(apiServerStore, mysqlStore)
	httpRoutes := repository.NewHTTPRoute(apiServerStore, mysqlStore)
	backends := repository.NewBackend(apiServerStore, mysqlStore)
	aiBackends := repository.NewAIBackend(apiServerStore, mysqlStore)
	modelRoutes := repository.NewModelRoute(apiServerStore, mysqlStore)
	mcpRoutes := repository.NewMCPRoute(apiServerStore, mysqlStore)

	gatewayService := gatewayservice.New(gateways)
	httpRouteService := httprouteservice.New(httpRoutes)
	backendService := backendservice.New(backends)
	aiBackendService := aibackendservice.New(aiBackends)
	modelRouteService := modelrouteservice.New(modelRoutes)
	mcpRouteService := mcprouteservice.New(mcpRoutes)

	gatewayHandler := handler.NewGateway(gatewayService)
	httpRouteHandler := handler.NewHTTPRoute(httpRouteService)
	backendHandler := handler.NewBackend(backendService)
	aiBackendHandler := handler.NewAIBackend(aiBackendService)
	modelRouteHandler := handler.NewModelRoute(modelRouteService)
	mcpRouteHandler := handler.NewMCPRoute(mcpRouteService)
	accountHandler := handler.NewAccount(accountService)

	handlers := server.Handlers{
		Gateway:    gatewayHandler,
		HTTPRoute:  httpRouteHandler,
		Backend:    backendHandler,
		AIBackend:  aiBackendHandler,
		ModelRoute: modelRouteHandler,
		MCPRoute:   mcpRouteHandler,
		Account:    accountHandler,
	}
	middlewareChain := server.NewMiddlewareChain(logger, middleware.RequireSession(accountService))
	httpServer, grpcServer := server.New(config.GetServer(), handlers, middlewareChain)

	return kratos.New(
		kratos.Context(ctx),
		kratos.Name(componentName),
		kratos.Version(version.Get().String()),
		kratos.Logger(logger),
		kratos.StopTimeout(shutdownTimeout),
		kratos.Server(httpServer, grpcServer),
	)
}

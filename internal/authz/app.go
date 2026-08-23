// Package authz 装配 ingate-authz 进程及其资源生命周期
package authz

import (
	"fmt"
	"log/slog"
	"os"

	kratos "github.com/go-kratos/kratos/v3"
	kratoslog "github.com/go-kratos/kratos/v3/log"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/authz/conf"
	dataapiserver "github.com/lgc202/ingate/internal/authz/data/apiserver"
	dataredis "github.com/lgc202/ingate/internal/authz/data/redis"
	"github.com/lgc202/ingate/internal/pkg/appconfig"
	"github.com/lgc202/ingate/internal/pkg/version"
)

const name = "ingate-authz"

type serviceInstanceID string

// App 封装 Authz 的 Kratos 进程
type App struct {
	kratos *kratos.App
}

// NewApp 从配置文件创建完整的 Authz 进程
func NewApp(configFile string) (*App, error) {
	var bootstrap conf.Bootstrap
	if err := appconfig.Load(configFile, &bootstrap); err != nil {
		return nil, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("read hostname: %w", err)
	}
	instanceID := serviceInstanceID(hostname)
	logger := appconfig.NewLogger(bootstrap.GetLogging(), name, string(instanceID))
	kratoslog.SetDefault(logger)
	kratosApp, err := wireApp(
		bootstrap.GetServer(),
		bootstrap.GetData().GetApiserver(),
		bootstrap.GetData().GetRedis(),
		logger,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("create Authz application: %w", err)
	}
	logger.Info("service starting", "config_file", configFile)
	return &App{kratos: kratosApp}, nil
}

// Run 启动 External Authorization gRPC 和运维 HTTP 服务
func (a *App) Run() error {
	return a.kratos.Run()
}

func newKratosApp(
	logger *slog.Logger,
	config *conf.Server,
	httpServer *kratoshttp.Server,
	grpcServer *kratosgrpc.Server,
	credentials *dataapiserver.CredentialCache,
	rates *dataredis.RateCounter,
	instanceID serviceInstanceID,
) *kratos.App {
	return kratos.New(
		kratos.ID(string(instanceID)),
		kratos.Name(name),
		kratos.Version(version.String()),
		kratos.Logger(logger),
		kratos.StopTimeout(config.GetShutdownTimeout().AsDuration()),
		// Caller 凭据缓存、请求计数器和网络服务共享同一进程生命周期
		kratos.Server(httpServer, grpcServer, credentials, rates),
	)
}

// Package adminapi 装配 ingate-admin-api 进程及其资源生命周期
package adminapi

import (
	"fmt"
	"log/slog"
	"os"

	kratos "github.com/go-kratos/kratos/v3"
	kratoslog "github.com/go-kratos/kratos/v3/log"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	"github.com/lgc202/ingate/internal/pkg/appconfig"
	"github.com/lgc202/ingate/internal/pkg/version"
)

const name = "ingate-admin-api"

type serviceInstanceID string

// App 封装 Admin API 的 Kratos 进程
type App struct {
	kratos  *kratos.App
	cleanup func()
}

// NewApp 从配置文件创建完整的 Admin API 进程
// 配置只在启动时读取，修改后需要重启组件才会生效
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

	kratosApp, cleanup, err := wireApp(
		bootstrap.GetServer(),
		bootstrap.GetData(),
		logger,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("create Admin API application: %w", err)
	}
	logger.Info("service starting", "config_file", configFile)
	return &App{kratos: kratosApp, cleanup: cleanup}, nil
}

// Run 启动 Admin API HTTP 服务
func (a *App) Run() error {
	defer a.cleanup()
	return a.kratos.Run()
}

func newKratosApp(
	logger *slog.Logger,
	config *conf.Server,
	httpServer *kratoshttp.Server,
	instanceID serviceInstanceID,
) *kratos.App {
	return kratos.New(
		kratos.ID(string(instanceID)),
		kratos.Name(name),
		kratos.Version(version.String()),
		kratos.Logger(logger),
		kratos.StopTimeout(config.GetShutdownTimeout().AsDuration()),
		kratos.Server(httpServer),
	)
}

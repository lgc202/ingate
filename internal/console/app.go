// Package console 装配 ingate-console 进程及其资源生命周期
package console

import (
	"fmt"
	"log/slog"
	"os"

	kratos "github.com/go-kratos/kratos/v3"
	kratoslog "github.com/go-kratos/kratos/v3/log"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/lgc202/go-kit/version"

	"github.com/lgc202/ingate/internal/console/conf"
	"github.com/lgc202/ingate/internal/pkg/appconfig"
)

const name = "ingate-console"

type serviceInstanceID string

// App 封装 Console 的 Kratos 进程
type App struct {
	kratos *kratos.App
}

// NewApp 从配置文件创建完整的 Console 进程
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
		bootstrap.GetData().GetAdminApi(),
		logger,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("create Console application: %w", err)
	}
	logger.Info("service starting", "config_file", configFile)
	return &App{kratos: kratosApp}, nil
}

// Run 启动控制台静态资源和管理 API 代理
func (a *App) Run() error {
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
		kratos.Version(version.Get().String()),
		kratos.Logger(logger),
		kratos.StopTimeout(config.GetShutdownTimeout().AsDuration()),
		kratos.Server(httpServer),
	)
}

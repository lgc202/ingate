// Package assistant 装配 ingate-assistant 进程及其资源生命周期。
package assistant

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	kratos "github.com/go-kratos/kratos/v3"
	kratoslog "github.com/go-kratos/kratos/v3/log"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/assistant/conf"
	"github.com/lgc202/ingate/internal/assistant/server"
	"github.com/lgc202/ingate/internal/pkg/appconfig"
	"github.com/lgc202/ingate/internal/pkg/version"
)

const name = "ingate-assistant"

type serviceInstanceID string

// App 封装 Kratos 进程和 Wire 创建的外部资源。
type App struct {
	kratos  *kratos.App
	cleanup func()
}

// NewApp 从配置文件创建包含 HTTP 与 Temporal Worker 的运维助手进程。
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
		context.Background(),
		bootstrap.GetServer(),
		bootstrap.GetData().GetMysql(),
		bootstrap.GetTemporal(),
		bootstrap.GetModel(),
		logger,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("create Assistant application: %w", err)
	}
	logger.Info("service starting", "config_file", configFile, "role", "all")
	return &App{kratos: kratosApp, cleanup: cleanup}, nil
}

// Run 启动运维助手，并在退出后释放 Temporal 与 MySQL 连接。
func (a *App) Run() error {
	defer a.cleanup()
	return a.kratos.Run()
}

func newKratosApp(
	logger *slog.Logger,
	config *conf.Server,
	httpServer *kratoshttp.Server,
	worker *server.Worker,
	instanceID serviceInstanceID,
) *kratos.App {
	return kratos.New(
		kratos.ID(string(instanceID)),
		kratos.Name(name),
		kratos.Version(version.String()),
		kratos.Logger(logger),
		kratos.StopTimeout(config.GetShutdownTimeout().AsDuration()),
		kratos.Server(httpServer, worker),
	)
}

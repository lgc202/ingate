// Package apiserver 装配 ingate-apiserver 进程及其资源生命周期
package apiserver

import (
	"fmt"
	"log/slog"
	"os"

	kratos "github.com/go-kratos/kratos/v3"
	kratoslog "github.com/go-kratos/kratos/v3/log"
	"k8s.io/klog/v2"

	"github.com/lgc202/ingate/internal/apiserver/conf"
	"github.com/lgc202/ingate/internal/apiserver/server"
	"github.com/lgc202/ingate/internal/pkg/appconfig"
	"github.com/lgc202/ingate/internal/pkg/version"
)

const name = "ingate-apiserver"

type serviceInstanceID string

// App 封装 API Server 的 Kratos 进程
type App struct {
	kratos *kratos.App
}

// NewApp 从配置文件创建完整的 API Server 进程
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
	klog.SetSlogLogger(logger)

	kratosApp, err := wireApp(
		bootstrap.GetServer(),
		bootstrap.GetServer().GetHttp(),
		bootstrap.GetData().GetEtcd(),
		logger,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("create API Server application: %w", err)
	}
	logger.Info("service starting", "config_file", configFile)
	return &App{kratos: kratosApp}, nil
}

// Run 启动 API Server 并由 Kratos 统一管理服务生命周期
func (a *App) Run() error {
	return a.kratos.Run()
}

func newKratosApp(
	logger *slog.Logger,
	config *conf.Server,
	apiServer *server.Server,
	instanceID serviceInstanceID,
) *kratos.App {
	return kratos.New(
		kratos.ID(string(instanceID)),
		kratos.Name(name),
		kratos.Version(version.String()),
		kratos.Logger(logger),
		kratos.StopTimeout(config.GetShutdownTimeout().AsDuration()),
		kratos.Server(apiServer),
	)
}

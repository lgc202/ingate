// Package adminapi 装配 ingate-admin-api 进程及其资源生命周期
package adminapi

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
	kratoslog "github.com/go-kratos/kratos/v3/log"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/lgc202/go-kit/version"

	"github.com/lgc202/ingate/internal/adminapi/conf"
)

const name = "ingate-admin-api"

type serviceInstanceID string

// App 封装 Admin API 的 Kratos 进程
type App struct {
	kratos *kratos.App
}

// NewApp 从配置文件创建完整的 Admin API 进程
// 配置只在启动时读取，修改后需要重启组件才会生效
func NewApp(configFile string) (*App, error) {
	var bootstrap conf.Bootstrap
	if err := loadConfig(configFile, &bootstrap); err != nil {
		return nil, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("read hostname: %w", err)
	}
	instanceID := serviceInstanceID(hostname)
	logger := newLogger(bootstrap.GetLogging(), string(instanceID))
	kratoslog.SetDefault(logger)

	kratosApp, err := wireApp(
		bootstrap.GetServer(),
		bootstrap.GetData(),
		bootstrap.GetAuthentication(),
		logger,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("create Admin API application: %w", err)
	}
	logger.Info("service starting", "config_file", configFile)
	return &App{kratos: kratosApp}, nil
}

// Run 启动 Admin API HTTP 服务
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

func loadConfig(configFile string, bootstrap *conf.Bootstrap) error {
	loaded := config.New(config.WithSource(file.NewSource(configFile)))
	defer loaded.Close()
	if err := loaded.Load(); err != nil {
		return fmt.Errorf("load configuration %q: %w", configFile, err)
	}
	if err := loaded.Scan(bootstrap); err != nil {
		return fmt.Errorf("scan configuration %q: %w", configFile, err)
	}
	if err := bootstrap.Validate(); err != nil {
		return fmt.Errorf("validate configuration %q: %w", configFile, err)
	}
	return nil
}

func newLogger(config *conf.Logging, instanceID string) *slog.Logger {
	format := kratoslog.FormatText
	if strings.EqualFold(config.GetFormat(), "json") {
		format = kratoslog.FormatJSON
	}
	handler := kratoslog.NewHandler(
		kratoslog.WithWriter(os.Stderr),
		kratoslog.WithFormat(format),
		kratoslog.WithLevel(kratoslog.ParseLevel(config.GetLevel())),
		kratoslog.WithAddSource(config.GetAddSource()),
	)
	return kratoslog.NewLogger(handler).With(
		"service.id", instanceID,
		"service.name", name,
		"service.version", version.Get().String(),
	)
}

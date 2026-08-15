// Package controller 装配 ingate-controller 进程及其资源生命周期
package controller

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
	kratoslog "github.com/go-kratos/kratos/v3/log"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/lgc202/go-kit/version"

	"github.com/lgc202/ingate/internal/controller/biz"
	"github.com/lgc202/ingate/internal/controller/biz/delivery"
	"github.com/lgc202/ingate/internal/controller/conf"
)

const name = "ingate-controller"

type serviceInstanceID string

// App 封装 Controller 的 Kratos 进程
type App struct {
	kratos *kratos.App
}

// NewApp 从配置文件创建完整的 Controller 进程
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
		bootstrap.GetData().GetApiserver(),
		bootstrap.GetDelivery(),
		bootstrap.GetResourceWatch(),
		logger,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("create Controller application: %w", err)
	}
	logger.Info("service starting", "config_file", configFile)
	return &App{kratos: kratosApp}, nil
}

// Run 启动 xDS、运维 HTTP、配置发布和资源收敛服务
func (a *App) Run() error {
	return a.kratos.Run()
}

func newKratosApp(
	logger *slog.Logger,
	config *conf.Server,
	httpServer *kratoshttp.Server,
	grpcServer *kratosgrpc.Server,
	configDelivery *delivery.Delivery,
	controller *biz.Controller,
	instanceID serviceInstanceID,
) *kratos.App {
	return kratos.New(
		kratos.ID(string(instanceID)),
		kratos.Name(name),
		kratos.Version(version.Get().String()),
		kratos.Logger(logger),
		kratos.StopTimeout(config.GetShutdownTimeout().AsDuration()),
		kratos.Server(httpServer, grpcServer, configDelivery, controller),
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

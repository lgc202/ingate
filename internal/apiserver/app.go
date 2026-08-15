// Package apiserver 装配 ingate-apiserver 进程及其资源生命周期
package apiserver

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	kratos "github.com/go-kratos/kratos/v3"
	kratosconfig "github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
	kratoslog "github.com/go-kratos/kratos/v3/log"
	"github.com/lgc202/go-kit/version"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"

	"github.com/lgc202/ingate/internal/apiserver/conf"
	"github.com/lgc202/ingate/internal/apiserver/registry"
	"github.com/lgc202/ingate/internal/apiserver/server"
	"github.com/lgc202/ingate/pkg/etcdx"
)

const name = "ingate-apiserver"

type serviceInstanceID string

// App 封装 API Server 的 Kratos 进程和外部资源
type App struct {
	kratos  *kratos.App
	cleanup func()
}

// NewApp 从配置文件创建完整的 API Server 进程
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
	klog.SetSlogLogger(logger)

	kratosApp, cleanup, err := wireApp(
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
	return &App{kratos: kratosApp, cleanup: cleanup}, nil
}

// Run 启动 API Server，退出后释放不由 Kratos Server 管理的连接
func (a *App) Run() error {
	defer a.cleanup()
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
		kratos.Version(version.Get().String()),
		kratos.Logger(logger),
		kratos.StopTimeout(config.GetShutdownTimeout().AsDuration()),
		kratos.Server(apiServer),
	)
}

func loadConfig(configFile string, bootstrap *conf.Bootstrap) error {
	loaded := kratosconfig.New(kratosconfig.WithSource(file.NewSource(configFile)))
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

func newServer(
	httpConfig *conf.Server_HTTP,
	etcdConfig *conf.Data_Etcd,
	logger *slog.Logger,
) (*server.Server, func(), error) {
	host, portText, err := net.SplitHostPort(httpConfig.GetAddr())
	if err != nil {
		return nil, nil, fmt.Errorf("parse API server address %q: %w", httpConfig.GetAddr(), err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return nil, nil, fmt.Errorf("parse API server port %q: %w", portText, err)
	}

	options := server.NewOptions()
	options.SecureServing.BindAddress = netutils.ParseIPSloppy(host)
	options.SecureServing.BindPort = port
	options.SecureServing.ServerCert.CertDirectory = httpConfig.GetCertDirectory()
	options.Etcd.StorageConfig.Transport.ServerList = append([]string(nil), etcdConfig.GetEndpoints()...)
	options.Etcd.StorageConfig.Prefix = etcdConfig.GetPrefix()
	if err := options.Complete(); err != nil {
		return nil, nil, fmt.Errorf("complete API server options: %w", err)
	}
	if err := options.Validate(); err != nil {
		return nil, nil, fmt.Errorf("validate API server options: %w", err)
	}

	serverConfig, err := options.Config()
	if err != nil {
		return nil, nil, fmt.Errorf("create API server configuration: %w", err)
	}
	storageConfig := etcdx.Config{
		Endpoints: append([]string(nil), etcdConfig.GetEndpoints()...),
		Prefix:    etcdConfig.GetPrefix(),
	}
	displayNameClient, err := etcdx.NewClient(storageConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("create etcd coordination client: %w", err)
	}
	serverConfig.DisplayNameGuard = registry.NewDisplayNameGuard(displayNameClient, storageConfig.Prefix)
	apiServer, err := serverConfig.Complete().New(genericapiserver.NewEmptyDelegate())
	if err != nil {
		if closeErr := displayNameClient.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close etcd coordination client: %w", closeErr))
		}
		return nil, nil, fmt.Errorf("create API server: %w", err)
	}
	cleanup := func() {
		if err := displayNameClient.Close(); err != nil {
			logger.Error("close etcd coordination client failed", "err", err)
		}
	}
	return apiServer, cleanup, nil
}

// Package als 装配 ingate-als 进程及其资源生命周期
//
// 组件的主链路为 Envoy ALS -> gRPC Service -> biz.Recorder -> Kafka，
// Kafka 不可用时先写入本地磁盘队列，DiskQueueReplayer 在恢复后重新投递积压记录
package als

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

	"github.com/lgc202/ingate/internal/als/conf"
	"github.com/lgc202/ingate/internal/als/server"
)

const name = "ingate-als"

type serviceInstanceID string

// App 封装 Kratos 进程和 Wire 创建的外部资源
type App struct {
	kratos  *kratos.App
	cleanup func()
}

// NewApp 从配置文件创建完整的 ALS 进程
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

	kratosApp, cleanup, err := wireApp(
		bootstrap.GetServer(),
		bootstrap.GetData().GetKafka(),
		bootstrap.GetData().GetDiskQueue(),
		logger,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("create ALS application: %w", err)
	}
	logger.Info("service starting", "config_file", configFile)
	return &App{kratos: kratosApp, cleanup: cleanup}, nil
}

// Run 启动 ALS 的 HTTP、gRPC 和磁盘队列回放，退出后释放 Kafka 和磁盘队列资源
func (a *App) Run() error {
	defer a.cleanup()
	return a.kratos.Run()
}

func newKratosApp(
	logger *slog.Logger,
	config *conf.Server,
	httpServer *kratoshttp.Server,
	grpcServer *kratosgrpc.Server,
	replayer *server.DiskQueueReplayer,
	instanceID serviceInstanceID,
) *kratos.App {
	// DiskQueueReplayer 实现 Kratos Server 接口，因此和 HTTP、gRPC 使用同一套生命周期
	return kratos.New(
		kratos.ID(string(instanceID)),
		kratos.Name(name),
		kratos.Version(version.Get().String()),
		kratos.Logger(logger),
		kratos.StopTimeout(config.GetShutdownTimeout().AsDuration()),
		kratos.Server(httpServer, grpcServer, replayer),
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

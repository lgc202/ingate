// Package analytics 装配 ingate-analytics 进程及其资源生命周期
//
// 组件的数据写入链路为 Kafka -> Consumer -> request.Recorder -> ClickHouse，
// 查询链路为 Admin API -> gRPC Service -> biz Queries -> ClickHouse。Kratos 只管理
// HTTP、gRPC 和 Consumer 的启动停止，不承载请求分析业务
package analytics

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	kratos "github.com/go-kratos/kratos/v3"
	kratoslog "github.com/go-kratos/kratos/v3/log"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/lgc202/go-kit/version"

	"github.com/lgc202/ingate/internal/analytics/conf"
	clickhousedata "github.com/lgc202/ingate/internal/analytics/data/clickhouse"
	"github.com/lgc202/ingate/internal/analytics/server"
	"github.com/lgc202/ingate/internal/pkg/appconfig"
)

const name = "ingate-analytics"

type serviceInstanceID string

// App 封装 Kratos 进程和 Wire 创建的外部资源
//
// Run 返回后会执行 cleanup，释放 ClickHouse 等不由 Kratos Server 管理的连接
type App struct {
	kratos  *kratos.App
	cleanup func()
}

// NewApp 从配置文件创建完整的 Analytics 进程
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
	logger := newLogger(bootstrap.GetLogging(), string(instanceID))
	kratoslog.SetDefault(logger)

	kratosApp, cleanup, err := wireApp(
		bootstrap.GetServer(),
		bootstrap.GetData().GetKafka(),
		bootstrap.GetData().GetClickHouse(),
		logger,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("create Analytics application: %w", err)
	}
	logger.Info("service starting", "config_file", configFile)
	return &App{kratos: kratosApp, cleanup: cleanup}, nil
}

// Run 启动 Analytics 的 HTTP、gRPC 和 Kafka Consumer，退出后释放 ClickHouse 连接
func (a *App) Run() error {
	defer a.cleanup()
	return a.kratos.Run()
}

// Migrate 应用 Analytics 的 ClickHouse 表结构变更后退出
func Migrate(ctx context.Context, configFile string) (int, error) {
	var bootstrap conf.Bootstrap
	if err := appconfig.Load(configFile, &bootstrap); err != nil {
		return 0, err
	}
	applied, err := clickhousedata.Migrate(ctx, bootstrap.GetData().GetClickHouse())
	if err != nil {
		return 0, err
	}
	return applied, nil
}

func newKratosApp(
	logger *slog.Logger,
	config *conf.Server,
	httpServer *kratoshttp.Server,
	grpcServer *kratosgrpc.Server,
	consumer *server.Consumer,
	instanceID serviceInstanceID,
) *kratos.App {
	// Consumer 实现 Kratos Server 接口，因此和 HTTP、gRPC 使用同一套生命周期
	return kratos.New(
		kratos.ID(string(instanceID)),
		kratos.Name(name),
		kratos.Version(version.Get().String()),
		kratos.Logger(logger),
		kratos.StopTimeout(config.GetShutdownTimeout().AsDuration()),
		kratos.Server(httpServer, grpcServer, consumer),
	)
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

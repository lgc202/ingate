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
	mysqldata "github.com/lgc202/ingate/internal/assistant/data/mysql"
	"github.com/lgc202/ingate/internal/assistant/worker"
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

// NewApp 从配置文件创建完整的运维助手进程。
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
		bootstrap.GetData().GetRedis(),
		bootstrap.GetStream(),
		bootstrap.GetWorker(),
		bootstrap.GetAdminApi(),
		logger,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("create Assistant application: %w", err)
	}
	logger.Info("service starting", "config_file", configFile)
	return &App{kratos: kratosApp, cleanup: cleanup}, nil
}

// Run 启动运维助手，并在退出后释放 MySQL 与 Redis 连接池。
func (a *App) Run() error {
	defer a.cleanup()
	return a.kratos.Run()
}

// Migrate 应用运维助手的 MySQL 表结构变更后退出。
func Migrate(ctx context.Context, configFile string) (int, error) {
	var bootstrap conf.Bootstrap
	if err := appconfig.Load(configFile, &bootstrap); err != nil {
		return 0, err
	}
	return mysqldata.Migrate(ctx, bootstrap.GetData().GetMysql())
}

func newKratosApp(
	logger *slog.Logger,
	config *conf.Server,
	httpServer *kratoshttp.Server,
	executionWorker *worker.ExecutionWorker,
	instanceID serviceInstanceID,
) *kratos.App {
	return kratos.New(
		kratos.ID(string(instanceID)),
		kratos.Name(name),
		kratos.Version(version.String()),
		kratos.Logger(logger),
		kratos.StopTimeout(config.GetShutdownTimeout().AsDuration()),
		kratos.Server(httpServer, executionWorker),
	)
}

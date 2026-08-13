// Package als 装配 ingate-als 进程的业务、数据和 transport 依赖
package als

import (
	"log/slog"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/lgc202/go-kit/version"

	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
	"github.com/lgc202/ingate/internal/als/data/diskqueue"
	"github.com/lgc202/ingate/internal/als/data/kafka"
	"github.com/lgc202/ingate/internal/als/server"
	"github.com/lgc202/ingate/internal/als/service"
)

// Name 是 ingate-als 的稳定进程名
const Name = "ingate-als"

// App 封装 ALS 进程及其 Kafka、WAL 资源生命周期
type App struct {
	kratos *kratos.App
	writer *kafka.Writer
	queue  *diskqueue.Queue
	logger *slog.Logger
}

// NewApp 创建完整的 ingate-als 进程
func NewApp(config *conf.Bootstrap, logger *slog.Logger, instanceID string) (*App, error) {
	writer, err := kafka.NewWriter(config.GetData().GetKafka())
	if err != nil {
		return nil, err
	}
	queue, err := diskqueue.NewQueue(config.GetData().GetDiskQueue())
	if err != nil {
		writer.Close()
		return nil, err
	}

	recorder := biz.NewRecorder(writer, queue, logger)
	alsService := service.NewService(recorder, logger)
	grpcServer, err := server.NewGRPCServer(config.GetServer(), alsService)
	if err != nil {
		writer.Close()
		_ = queue.Close()
		return nil, err
	}
	httpServer := server.NewHTTPServer(config, writer, recorder)
	backlog := server.NewBacklog(config.GetData(), recorder, logger)
	kratosApp := kratos.New(
		kratos.ID(instanceID),
		kratos.Name(Name),
		kratos.Version(version.Get().String()),
		kratos.Logger(logger),
		kratos.StopTimeout(config.GetServer().GetShutdownTimeout().AsDuration()),
		kratos.Server(grpcServer, httpServer, backlog),
	)

	return &App{kratos: kratosApp, writer: writer, queue: queue, logger: logger}, nil
}

// Run 运行所有 transport，并在退出后释放 Kafka 和 WAL 资源
func (a *App) Run() error {
	err := a.kratos.Run()
	a.writer.Close()
	if closeErr := a.queue.Close(); closeErr != nil {
		a.logger.Error("close disk queue failed", "error", closeErr)
		if err == nil {
			return closeErr
		}
	}
	return err
}

// Package als 装配 ingate-als 进程及其资源生命周期。
//
// 组件的主链路为 Envoy ALS -> gRPC Service -> biz.Recorder -> Kafka，
// Kafka 不可用时先写入本地磁盘队列，DiskQueueReplayer 在恢复后重新投递积压记录。
package als

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	kratos "github.com/go-kratos/kratos/v3"
	kratoslog "github.com/go-kratos/kratos/v3/log"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
	"github.com/lgc202/ingate/internal/als/server"
	"github.com/lgc202/ingate/internal/pkg/appconfig"
	"github.com/lgc202/ingate/internal/pkg/telemetry"
	"github.com/lgc202/ingate/internal/pkg/tlsconfig"
	"github.com/lgc202/ingate/internal/pkg/version"
)

const (
	name       = "ingate-als"
	tracerName = "github.com/lgc202/ingate/internal/als"
)

type serviceInstanceID string

// App 封装 Kratos 进程和 Wire 创建的外部资源。
type App struct {
	kratos          *kratos.App
	tracing         *telemetry.Tracing
	cleanup         func()
	shutdownTimeout time.Duration
}

// NewApp 从配置文件创建完整的 ALS 进程。
// 配置只在启动时读取，修改后需要重启组件才会生效。
func NewApp(configFile string) (*App, error) {
	var bootstrap conf.Bootstrap
	if err := appconfig.Load(configFile, &bootstrap); err != nil {
		return nil, err
	}

	identity, err := telemetry.NewIdentity(name, bootstrap.GetTelemetry().GetEnvironment())
	if err != nil {
		return nil, err
	}

	instanceID := serviceInstanceID(identity.InstanceID)
	logger := telemetry.NewLogger(bootstrap.GetLogging(), identity)
	kratoslog.SetDefault(logger)

	traceConfig, err := tracingConfig(bootstrap.GetTelemetry().GetTracing())
	if err != nil {
		return nil, err
	}

	tracing, err := telemetry.NewTracing(context.Background(), traceConfig, identity)
	if err != nil {
		return nil, err
	}

	kratosApp, cleanup, err := wireApp(
		bootstrap.GetServer(),
		bootstrap.GetData(),
		logger,
		tracing,
		instanceID,
	)
	if err != nil {
		shutdownErr := shutdownTracing(tracing, bootstrap.GetServer().GetShutdownTimeout().AsDuration())
		return nil, errors.Join(
			fmt.Errorf("initialize ALS dependencies: %w", err),
			shutdownErr,
		)
	}

	return &App{
		kratos:          kratosApp,
		tracing:         tracing,
		cleanup:         cleanup,
		shutdownTimeout: bootstrap.GetServer().GetShutdownTimeout().AsDuration(),
	}, nil
}

// Run 启动 ALS 的 HTTP、gRPC 和磁盘队列回放，退出后释放 Kafka 和磁盘队列资源。
func (a *App) Run() error {
	runErr := a.kratos.Run()
	a.cleanup()
	return errors.Join(runErr, shutdownTracing(a.tracing, a.shutdownTimeout))
}

func tracingConfig(config *conf.Telemetry_Tracing) (telemetry.TraceConfig, error) {
	endpoint := strings.TrimSpace(config.GetEndpoint())
	var clientTLS *tls.Config
	var err error
	if endpoint != "" && !config.GetInsecure() {
		tlsSettings := config.GetTls()
		clientTLS, err = tlsconfig.NewClient(tlsconfig.ClientConfig{
			Enabled:         true,
			CAFile:          tlsSettings.GetCaFile(),
			CertificateFile: tlsSettings.GetCertFile(),
			PrivateKeyFile:  tlsSettings.GetKeyFile(),
			ServerName:      tlsSettings.GetServerName(),
		})
		if err != nil {
			return telemetry.TraceConfig{}, fmt.Errorf("create telemetry tracing TLS config: %w", err)
		}
	}

	return telemetry.TraceConfig{
		Endpoint:      endpoint,
		Insecure:      config.GetInsecure(),
		TLS:           clientTLS,
		SampleRatio:   config.GetSampleRatio(),
		QueueSize:     int(config.GetMaxQueueSize()),
		BatchSize:     int(config.GetExportBatchSize()),
		BatchTimeout:  config.GetBatchTimeout().AsDuration(),
		ExportTimeout: config.GetExportTimeout().AsDuration(),
	}, nil
}

func shutdownTracing(tracing *telemetry.Tracing, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := tracing.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown tracing: %w", err)
	}
	return nil
}

func newKratosApp(
	logger *slog.Logger,
	config *conf.Server,
	httpServer *kratoshttp.Server,
	grpcServer *kratosgrpc.Server,
	replayer *server.DiskQueueReplayer,
	topicMonitor *server.TopicMonitor,
	instanceID serviceInstanceID,
) *kratos.App {
	// 后台任务实现 Kratos Server 接口，因此和 HTTP、gRPC 使用同一套生命周期。
	return kratos.New(
		kratos.ID(string(instanceID)),
		kratos.Name(name),
		kratos.Version(version.String()),
		kratos.Logger(logger),
		kratos.StopTimeout(config.GetShutdownTimeout().AsDuration()),
		kratos.BeforeStart(topicMonitor.BeforeStart),
		kratos.Server(httpServer, grpcServer, replayer, topicMonitor),
	)
}

func newTopicContract(mode conf.Data_ReliabilityMode) *biz.TopicContract {
	reliability := biz.ReliabilityDevelopment
	if mode == conf.Data_PRODUCTION {
		reliability = biz.ReliabilityProduction
	}

	return biz.NewTopicContract(reliability)
}

func newTracer(tracing *telemetry.Tracing) oteltrace.Tracer {
	return tracing.Provider().Tracer(tracerName)
}

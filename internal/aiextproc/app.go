// Package aiextproc 装配 ingate-ai-extproc 进程及其资源生命周期
//
// Envoy 的 downstream ExtProc 负责提取客户端模型并处理最终响应，upstream ExtProc
// 在负载均衡完成后注入凭据并转换厂商协议；两个独立流通过进程内请求状态关联
package aiextproc

import (
	"fmt"
	"log/slog"
	"os"

	kratos "github.com/go-kratos/kratos/v3"
	kratoslog "github.com/go-kratos/kratos/v3/log"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/lgc202/ingate/internal/aiextproc/conf"
	dataapiserver "github.com/lgc202/ingate/internal/aiextproc/data/apiserver"
	"github.com/lgc202/ingate/internal/pkg/appconfig"
	"github.com/lgc202/ingate/internal/pkg/version"
)

const name = "ingate-ai-extproc"

type serviceInstanceID string

// App 封装 AI ExtProc 的 Kratos 进程
type App struct {
	kratos *kratos.App
}

// NewApp 从配置文件创建完整的 AI ExtProc 进程
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

	kratosApp, err := wireApp(
		bootstrap.GetServer(),
		bootstrap.GetData().GetApiserver(),
		logger,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("create AI ExtProc application: %w", err)
	}
	logger.Info("service starting", "config_file", configFile)
	return &App{kratos: kratosApp}, nil
}

// Run 启动 External Processing gRPC 和运维 HTTP 服务
func (a *App) Run() error {
	return a.kratos.Run()
}

func newKratosApp(
	logger *slog.Logger,
	config *conf.Server,
	httpServer *kratoshttp.Server,
	grpcServer *kratosgrpc.Server,
	modelServices *dataapiserver.ModelServiceCache,
	instanceID serviceInstanceID,
) *kratos.App {
	return kratos.New(
		kratos.ID(string(instanceID)),
		kratos.Name(name),
		kratos.Version(version.String()),
		kratos.Logger(logger),
		kratos.StopTimeout(config.GetShutdownTimeout().AsDuration()),
		// 模型服务缓存和网络服务共享生命周期，首次同步完成前 /readyz 保持未就绪
		kratos.Server(httpServer, grpcServer, modelServices),
	)
}

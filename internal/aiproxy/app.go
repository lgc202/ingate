// Package aiproxy 装配 AI Proxy 的模型请求链，管理组件启动与配置 Watch 生命周期。
// 本包不拥有领域资源或配置编译。
package aiproxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/go-kratos/kratos/v3"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/aiproxy/conf"
	"github.com/lgc202/ingate/internal/aiproxy/handler"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/envoyclient"
	"github.com/lgc202/ingate/internal/aiproxy/server"
	"github.com/lgc202/ingate/internal/aiproxy/service/modelproxy"
	logconfig "github.com/lgc202/ingate/internal/pkg/log"
	"github.com/lgc202/ingate/internal/pkg/version"
)

const componentName = "ingate-ai-proxy"

// Run 加载启动配置并运行唯一的 Kratos App，直到 Context 取消或框架收到停止信号。
// HTTP 先停止接收并有界排空，再取消、等待 Loader，最后清理两个 HTTP 连接池。
// socket 目录由部署侧准备；不会删除启动前已有的路径或创建 Namespace/ConfigMap。
func Run(ctx context.Context, configFile string) (runErr error) {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("start AI Proxy: %w", err)
	}

	config, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("load AI Proxy config: %w", err)
	}

	logger, closeLog, err := logconfig.New(logconfig.Config{})
	if err != nil {
		return fmt.Errorf("initialize AI Proxy log: %w", err)
	}
	defer func() {
		if err := closeLog(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close AI Proxy log: %w", err))
		}
	}()
	logger = logger.With("service", componentName)

	loader, closeAPI, err := newConfigLoader(config.GetApiServer(), logger)
	if err != nil {
		return err
	}
	defer closeAPI()

	listener, err := listenUnix(config.GetServer().GetSocket())
	if err != nil {
		return err
	}

	app, closeIdle, err := newApp(ctx, config, listener, loader, logger)
	if err != nil {
		// 尚未移交给 Server；监听器的失败清理仍由取得它的 Run 负责。
		return errors.Join(err, listener.Close())
	}
	defer closeIdle()

	if err := runApp(ctx, app, loader); err != nil {
		return fmt.Errorf("run AI Proxy: %w", err)
	}

	return nil
}

// NewHTTPServer 将原生出站 Client、完整模型 Service 和 Handler 装配到 Kratos HTTP Server。
// 调用方拥有 Listener 与 Loader：负责保护入口 socket 及目录、运行 Loader，
// 并在停止接收、排空 Server 后调用返回的清理函数释放出口空闲连接。
// 构造不开始接受请求或启动 Watch；失败不关闭调用方的 Listener 或 Loader。
// Listener、Loader 与 Logger 由调用方的成功构造链保证有效。
// Server 从 Envoy 覆盖的内部 Header 恢复入口；入口信任依赖调用方保护 Unix socket。
func NewHTTPServer(
	listener *net.UnixListener,
	loader *aiconfig.Loader,
	egressSocket string,
	maxRequestBodyBytes int64,
	logger *slog.Logger,
) (*kratoshttp.Server, func(), error) {
	client, err := envoyclient.New(egressSocket)
	if err != nil {
		return nil, nil, fmt.Errorf("create Envoy HTTP client: %w", err)
	}

	service := modelproxy.New(loader, client)
	modelProxyHandler, err := handler.NewModelProxy(service, maxRequestBodyBytes, logger)
	if err != nil {
		client.CloseIdleConnections()
		return nil, nil, fmt.Errorf("create model handler: %w", err)
	}

	httpServer := server.New(listener, server.Handlers{ModelProxy: modelProxyHandler})

	return httpServer, client.CloseIdleConnections, nil
}

func newApp(
	ctx context.Context,
	config *conf.Bootstrap,
	listener *net.UnixListener,
	loader *aiconfig.Loader,
	logger *slog.Logger,
) (*kratos.App, func(), error) {
	httpServer, closeIdle, err := NewHTTPServer(
		listener,
		loader,
		config.GetEnvoy().GetEgressSocket(),
		config.GetServer().GetMaxRequestBodyBytes(),
		logger,
	)
	if err != nil {
		return nil, nil, err
	}

	app := kratos.New(
		kratos.Context(ctx),
		kratos.Name(componentName),
		kratos.Version(version.Get().String()),
		kratos.Logger(logger),
		kratos.StopTimeout(config.GetServer().GetShutdownTimeout().AsDuration()),
		kratos.Server(httpServer),
	)

	return app, closeIdle, nil
}

func runApp(ctx context.Context, app *kratos.App, loader *aiconfig.Loader) error {
	// 正常信号退出时，Watch 活到 HTTP 排空结束；其生命周期仍由本次运行拥有。
	// WithoutCancel 只解除父取消，下面的显式取消与等待覆盖 App 的正常和失败返回。
	watchCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()
	done := make(chan error, 1)
	go func() {
		err := loader.Run(watchCtx)
		if err != nil && !errors.Is(err, context.Canceled) {
			err = errors.Join(err, app.Stop())
		}
		done <- err
	}()

	appErr := app.Run()
	cancel()
	loaderErr := <-done
	if errors.Is(loaderErr, context.Canceled) {
		loaderErr = nil
	}

	return errors.Join(appErr, loaderErr)
}

func newConfigLoader(config *conf.APIServer, logger *slog.Logger) (*aiconfig.Loader, func(), error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", config.GetKubeconfig())
	if err != nil {
		return nil, nil, fmt.Errorf("build Ingate API client configuration: %w", err)
	}
	restConfig = rest.AddUserAgent(restConfig, componentName)

	apiHTTP, err := rest.HTTPClientFor(restConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("create Ingate API HTTP client: %w", err)
	}

	apiClient, err := kubernetes.NewForConfigAndClient(restConfig, apiHTTP)
	if err != nil {
		apiHTTP.CloseIdleConnections()
		return nil, nil, fmt.Errorf("create Ingate API client: %w", err)
	}

	loader, err := aiconfig.NewLoader(apiClient, config.GetAiConfigNamespace(), logger)
	if err != nil {
		apiHTTP.CloseIdleConnections()
		return nil, nil, fmt.Errorf("create AI configuration loader: %w", err)
	}

	return loader, apiHTTP.CloseIdleConnections, nil
}

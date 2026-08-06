// Package app 负责 ingate-ai-proxy 进程的配置加载和服务装配
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	kitconfig "github.com/lgc202/go-kit/config"
	"github.com/lgc202/go-kit/version"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/lgc202/ingate/internal/aiproxy/extproc"
	"github.com/lgc202/ingate/internal/pkg/aiproxyconfig"
	"github.com/lgc202/ingate/internal/pkg/httpserver"
	"github.com/lgc202/ingate/pkg/redisx"
)

const usage = `ingate-ai-proxy 处理经过 Envoy 的 AI 请求

职责：
  - 通过 Envoy ExtProc 接收 AI 请求上下文
  - 校验客户端访问密钥和公开模型权限
  - 执行模型选路、厂商协议转换和响应归一化
`

// Run 执行 ingate-ai-proxy 服务
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := pflag.NewFlagSet("ingate-ai-proxy", pflag.ContinueOnError)
	flags.SetOutput(stderr)

	configPath := defaultConfigPath
	showVersion := false
	flags.StringVar(&configPath, "config", configPath, "配置文件路径")
	flags.BoolVar(&showVersion, "version", showVersion, "显示版本信息")
	flags.Usage = func() {
		fmt.Fprint(stdout, usage)
		fmt.Fprintln(stdout)
		flags.SetOutput(stdout)
		flags.PrintDefaults()
		flags.SetOutput(stderr)
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil
		}
		return err
	}
	if showVersion {
		fmt.Fprintln(stdout, version.Get().Text())
		return nil
	}

	loaded, err := kitconfig.Load[Config](configPath, kitconfig.WithEnv[Config]("INGATE_AI_PROXY"))
	if err != nil {
		return err
	}
	settings := loaded.Get()
	if err := settings.Validate(); err != nil {
		return err
	}

	logger, err := settings.Logging.NewLogger("ingate-ai-proxy", stdout)
	if err != nil {
		return err
	}
	defer logger.Close()
	loaded.OnChange(func(old, next Config) {
		if err := next.Validate(); err != nil {
			logger.Error("ignoring invalid configuration change", "err", err)
			return
		}
		old.Logging.ApplyDynamic(next.Logging, logger)
		old.Logging = next.Logging
		if kitconfig.Changed(old, next) {
			logger.Warn("service configuration changed and requires a restart")
		}
	})
	logger.Info("service started", "config_file", configPath)

	return serve(ctx, settings, logger.Logger)
}

func serve(ctx context.Context, settings Config, logger *slog.Logger) error {
	redisClient, err := redisx.NewClient(ctx, settings.Redis)
	if err != nil {
		return err
	}
	defer redisClient.Close()

	grpcAddress := net.JoinHostPort(settings.Server.GRPC.Address, strconv.Itoa(settings.Server.GRPC.Port))
	listener, err := net.Listen("tcp", grpcAddress)
	if err != nil {
		return fmt.Errorf("listen for AI ExtProc on %q: %w", grpcAddress, err)
	}
	defer listener.Close()

	processor := extproc.NewServer(redisClient, logger.With("component", "extproc"))
	httpAddress := net.JoinHostPort(settings.Server.HTTP.Address, strconv.Itoa(settings.Server.HTTP.Port))
	healthServer := httpserver.New(
		httpAddress,
		newHealthHandler(redisClient),
		logger.With("component", "health"),
	)

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return serveGRPC(groupCtx, listener, processor, logger.With("component", "extproc"))
	})
	group.Go(func() error {
		return healthServer.Run(groupCtx)
	})
	return group.Wait()
}

func serveGRPC(
	ctx context.Context,
	listener net.Listener,
	processor extprocv3.ExternalProcessorServer,
	logger *slog.Logger,
) error {
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(aiproxyconfig.ResponseBufferLimitBytes),
		grpc.MaxSendMsgSize(aiproxyconfig.ResponseBufferLimitBytes),
	)
	extprocv3.RegisterExternalProcessorServer(grpcServer, processor)

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("AI ExtProc gRPC server started", "addr", listener.Addr().String())
		serverErr <- grpcServer.Serve(listener)
	}()

	select {
	case err := <-serverErr:
		if err == nil {
			return errors.New("AI ExtProc gRPC server stopped unexpectedly")
		}
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return fmt.Errorf("serve AI ExtProc gRPC: %w", err)
	case <-ctx.Done():
		grpcServer.Stop()
		if err := <-serverErr; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("serve AI ExtProc gRPC: %w", err)
		}
		return nil
	}
}

func newHealthHandler(redisClient *redis.Client) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /readyz", func(writer http.ResponseWriter, request *http.Request) {
		if err := redisClient.Ping(request.Context()).Err(); err != nil {
			http.Error(writer, "not ready", http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ok\n"))
	})
	return mux
}

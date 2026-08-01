// Package app 实现 ingate-controller 的唯一装配入口
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"

	kitconfig "github.com/lgc202/go-kit/config"
	"github.com/lgc202/go-kit/version"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/controller/delivery"
	"github.com/lgc202/ingate/internal/controller/health"
	"github.com/lgc202/ingate/internal/controller/reconcile"
	"github.com/lgc202/ingate/internal/controller/xds"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

const usage = `ingate-controller 负责将声明式资源收敛为 Envoy 配置

职责：
  - 监听 ingate-apiserver 中的声明式资源
  - 编译并发布完整 Envoy xDS 配置
  - 处理 Envoy ACK/NACK 和配置回滚
  - 提供 ADS 和内部健康检查接口
`

// Run 解析启动参数并运行 ingate-controller
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := pflag.NewFlagSet("ingate-controller", pflag.ContinueOnError)
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

	defaults := DefaultConfig()
	loaded, err := kitconfig.Load[Config](configPath,
		kitconfig.WithDefaults[Config](map[string]any{
			"delivery.candidate_ack_timeout": defaults.Delivery.CandidateACKTimeout,
			"delivery.nack_rollback_timeout": defaults.Delivery.NACKRollbackTimeout,
		}),
		kitconfig.WithEnv[Config]("INGATE_CONTROLLER"),
	)
	if err != nil {
		return err
	}
	settings := loaded.Get()
	if err := settings.Validate(); err != nil {
		return err
	}

	logger, err := settings.Logging.NewLogger("ingate-controller", stdout)
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

	return run(ctx, settings, logger.Logger)
}

func run(ctx context.Context, settings Config, logger *slog.Logger) error {
	restConfig, err := clientcmd.BuildConfigFromFlags(settings.APIServer.Master, settings.APIServer.Kubeconfig)
	if err != nil {
		return fmt.Errorf("build apiserver client config: %w", err)
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("create apiserver resource client: %w", err)
	}
	xdsLogger := xds.NewSlogLogger(logger.With("component", "xds"))
	snapshotCache := xds.NewSnapshotCache(xdsLogger)
	configDelivery, err := delivery.New(snapshotCache, delivery.Options{
		ACKTimeout:          settings.Delivery.CandidateACKTimeout,
		NACKRollbackTimeout: settings.Delivery.NACKRollbackTimeout,
	})
	if err != nil {
		return fmt.Errorf("create config delivery: %w", err)
	}

	reconciler, err := reconcile.New(
		client,
		settings.ResourceWatch.ResyncPeriod,
		configDelivery,
		logger.With("component", "reconcile"),
	)
	if err != nil {
		return fmt.Errorf("create resource reconciler: %w", err)
	}
	healthServer := health.NewServer(logger.With("component", "health"))

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	group, groupCtx := errgroup.WithContext(runCtx)
	group.Go(func() error {
		return configDelivery.Run(groupCtx)
	})

	if ctx.Err() != nil {
		cancel()
		_ = group.Wait()
		return nil
	}
	xdsListener, err := net.Listen("tcp", settings.Server.XDSListenAddress)
	if err != nil {
		cancel()
		_ = group.Wait()
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("listen for xDS on %q: %w", settings.Server.XDSListenAddress, err)
	}
	defer xdsListener.Close()

	healthListener, err := net.Listen("tcp", settings.Server.HealthListenAddress)
	if err != nil {
		cancel()
		_ = group.Wait()
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("listen for controller health checks on %q: %w", settings.Server.HealthListenAddress, err)
	}
	defer healthListener.Close()

	callbacks := xds.NewCallbacks(configDelivery.HandleXDSEvent)
	adsServer := xds.NewServer(groupCtx, snapshotCache, callbacks, xdsLogger)

	group.Go(func() error {
		return adsServer.Serve(groupCtx, xdsListener, logger.With("component", "xds"))
	})
	group.Go(func() error {
		return healthServer.Serve(groupCtx, healthListener)
	})

	// Delivery 已运行且两个 listener 均已绑定，此时 Envoy 可以连接并等待首次编译结果
	healthServer.MarkReady()
	group.Go(func() error {
		return reconciler.Run(groupCtx)
	})

	err = group.Wait()
	if err != nil {
		return err
	}
	return nil
}

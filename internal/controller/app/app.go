// Package app 实现 ingate-controller 的唯一装配入口
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/controller/reconcile"
	controllerstatus "github.com/lgc202/ingate/internal/controller/status"
	"github.com/lgc202/ingate/internal/envoy/delivery"
	"github.com/lgc202/ingate/internal/envoy/xds"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	"github.com/lgc202/ingate/pkg/xlog"
)

const usage = `ingate-controller 负责将声明式资源收敛为 Envoy 配置

职责：
  - 监听 ingate-apiserver 中的声明式资源
  - 编译并发布完整 Envoy xDS 配置
  - 处理 Envoy ACK/NACK 和配置回滚
  - 提供 ADS 和内部运行状态接口
`

// Run 解析启动参数并运行 ingate-controller
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := pflag.NewFlagSet("ingate-controller", pflag.ContinueOnError)
	flags.SetOutput(stderr)

	options := NewOptions()
	options.AddFlags(flags)
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
	if err := options.Validate(); err != nil {
		return err
	}

	logger, err := xlog.New(xlog.Options{
		Output: logOutput(options.LogStdout, stdout),
		Format: options.LogFormat,
		Level:  options.LogLevel,
		File:   options.LogFile,
	})
	if err != nil {
		return err
	}
	defer logger.Close()

	return run(ctx, options, logger.Logger)
}

func run(ctx context.Context, options *Options, logger *slog.Logger) error {
	restConfig, err := clientcmd.BuildConfigFromFlags(options.Master, options.Kubeconfig)
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
		ACKTimeout:          options.CandidateACKTimeout,
		NACKRollbackTimeout: options.NACKRollbackTimeout,
	})
	if err != nil {
		return fmt.Errorf("create config delivery: %w", err)
	}

	runtimeStatus := controllerstatus.NewRuntime()
	reconciler, err := reconcile.New(
		client,
		options.ResyncPeriod,
		configDelivery,
		runtimeStatus,
		logger.With("component", "reconciler"),
	)
	if err != nil {
		return fmt.Errorf("create resource reconciler: %w", err)
	}
	internalServer := controllerstatus.NewServer(
		runtimeStatus,
		configDelivery,
		logger.With("component", "status"),
	)

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
	xdsListener, err := net.Listen("tcp", options.XDSListenAddress)
	if err != nil {
		cancel()
		_ = group.Wait()
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("listen for xDS on %q: %w", options.XDSListenAddress, err)
	}
	defer xdsListener.Close()

	internalListener, err := net.Listen("tcp", options.InternalListenAddress)
	if err != nil {
		cancel()
		_ = group.Wait()
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("listen for controller status on %q: %w", options.InternalListenAddress, err)
	}
	defer internalListener.Close()

	callbacks := xds.NewCallbacks(func(eventCtx context.Context, event xds.Event) error {
		return configDelivery.HandleXDSEvent(eventCtx, event)
	})
	adsServer := xds.NewServer(groupCtx, snapshotCache, callbacks, xdsLogger)

	group.Go(func() error {
		return adsServer.Serve(groupCtx, xdsListener, logger.With("component", "xds"))
	})
	group.Go(func() error {
		return internalServer.Serve(groupCtx, internalListener)
	})

	// Delivery 已运行且两个 listener 均已绑定，此时 Envoy 可以连接并等待首次编译结果
	internalServer.MarkReady()
	group.Go(func() error {
		return reconciler.Run(groupCtx)
	})

	err = group.Wait()
	if err != nil {
		return err
	}
	return nil
}

func logOutput(enabled bool, stdout io.Writer) io.Writer {
	if !enabled {
		return nil
	}
	return stdout
}

package app

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"

	appoptions "github.com/lgc202/ingate/cmd/xds-server/app/options"
	xdscache "github.com/lgc202/ingate/internal/controlplane/xds/cache"
	xdsconfig "github.com/lgc202/ingate/internal/controlplane/xds/config"
	controllerindex "github.com/lgc202/ingate/internal/controlplane/controller/index"
	controllerruntime "github.com/lgc202/ingate/internal/controlplane/controller/runtime"
	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	"github.com/lgc202/ingate/internal/controlplane/xds/publish"
	xdswatch "github.com/lgc202/ingate/internal/controlplane/xds/watch"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	externalversions "github.com/lgc202/ingate/pkg/generated/informers/externalversions"
	"k8s.io/client-go/util/workqueue"
)

const (
	xdsServerName       = "ingate-xds-server"
	defaultResyncPeriod = 30 * time.Second
	defaultQPS          = 20
	defaultBurst        = 40
)

func NewXDSServerCommand() *cobra.Command {
	o := appoptions.NewServerRunOptions()

	cmd := &cobra.Command{
		Use:          xdsServerName,
		Short:        "Launch the Ingate xDS server",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			completed, err := o.Complete()
			if err != nil {
				return err
			}
			if errs := completed.Validate(); len(errs) != 0 {
				return utilerrors.NewAggregate(errs)
			}
			return Run(cmd.Context(), cmd.OutOrStdout(), completed)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if arg != "" {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), arg)
				}
			}
			return nil
		},
	}

	fss := o.Flags()
	fs := cmd.Flags()
	for _, f := range fss.FlagSets {
		fs.AddFlagSet(f)
	}
	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cliflag.SetUsageAndHelpFunc(cmd, fss, cols)

	return cmd
}

func Run(ctx context.Context, out io.Writer, opts appoptions.CompletedOptions) error {
	if out == nil {
		out = io.Discard
	}

	cfg, err := xdsconfig.NewConfig(opts)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "starting %s\n", xdsServerName)
	fmt.Fprintf(out, "  apiserver-address: %s\n", cfg.Options.APIServerAddress)
	fmt.Fprintf(out, "  kubeconfig: %s\n", cfg.Options.Kubeconfig)
	fmt.Fprintf(out, "  grpc-bind-address: %s\n", cfg.Options.GRPCBindAddress)
	fmt.Fprintf(out, "  healthz-bind-address: %s\n", cfg.Options.HealthzBindAddress)
	fmt.Fprintf(out, "  watch-namespace: %q\n", cfg.Options.WatchNamespace)

	client, err := newClientset(cfg.Options)
	if err != nil {
		return err
	}

	informerFactory := newInformerFactory(client, cfg.Options)
	runtimeCache := xdscache.New()
	publisher := publish.NewServer(cfg.Options.GRPCBindAddress, runtimeCache, client)
	runtimeContext := controllerruntime.NewContext(client, informerFactory, controllerindex.New(), workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[shared.ObjectKey]()))
	watcher := xdswatch.NewGatewayWatcher(ctx, runtimeContext, publisher)
	if err := watcher.Register(); err != nil {
		return err
	}

	go informerFactory.Start(ctx.Done())
	if !cache.WaitForCacheSync(
		ctx.Done(),
		watcher.HasSynced,
		informerFactory.Gateway().V1alpha1().Routes().Informer().HasSynced,
		informerFactory.Gateway().V1alpha1().Backends().Informer().HasSynced,
		informerFactory.Gateway().V1alpha1().Certificates().Informer().HasSynced,
		informerFactory.Policy().V1alpha1().AuthPolicies().Informer().HasSynced,
		informerFactory.Policy().V1alpha1().TrafficPolicies().Informer().HasSynced,
	) {
		return fmt.Errorf("timed out waiting for xds informer caches to sync")
	}

	errCh := make(chan error, 2)
	go func() {
		errCh <- publisher.Run(ctx)
	}()
	go func() {
		errCh <- cfg.HealthServer.Run(ctx)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return nil
	}
}

func newClientset(opts appoptions.CompletedOptions) (clientset.Interface, error) {
	restConfig, err := newRESTConfig(opts)
	if err != nil {
		return nil, err
	}
	return clientset.NewForConfig(restConfig)
}

func newRESTConfig(opts appoptions.CompletedOptions) (*rest.Config, error) {
	if opts.Kubeconfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags(opts.APIServerAddress, opts.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("build kubeconfig-based xds-server rest config: %w", err)
		}
		cfg.UserAgent = xdsServerName
		cfg.QPS = defaultQPS
		cfg.Burst = defaultBurst
		return cfg, nil
	}

	return &rest.Config{
		Host:      opts.APIServerAddress,
		UserAgent: xdsServerName,
		QPS:       defaultQPS,
		Burst:     defaultBurst,
	}, nil
}

func newInformerFactory(client clientset.Interface, opts appoptions.CompletedOptions) externalversions.SharedInformerFactory {
	if opts.WatchNamespace != "" {
		return externalversions.NewSharedInformerFactoryWithOptions(
			client,
			defaultResyncPeriod,
			externalversions.WithNamespace(opts.WatchNamespace),
		)
	}
	return externalversions.NewSharedInformerFactory(client, defaultResyncPeriod)
}

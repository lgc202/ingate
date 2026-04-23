package app

import (
	"context"
	"fmt"
	"io"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	appoptions "github.com/lgc202/ingate/cmd/controller-manager/app/options"
	"github.com/lgc202/ingate/cmd/controller-manager/names"
	"github.com/lgc202/ingate/internal/controlplane/controller/authpolicy"
	"github.com/lgc202/ingate/internal/controlplane/controller/backend"
	"github.com/lgc202/ingate/internal/controlplane/controller/certificate"
	controllerconfig "github.com/lgc202/ingate/internal/controlplane/controller/config"
	controllergateway "github.com/lgc202/ingate/internal/controlplane/controller/gateway"
	controllerindex "github.com/lgc202/ingate/internal/controlplane/controller/index"
	"github.com/lgc202/ingate/internal/controlplane/controller/resolvedgateway"
	"github.com/lgc202/ingate/internal/controlplane/controller/route"
	controllerruntime "github.com/lgc202/ingate/internal/controlplane/controller/runtime"
	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	"github.com/lgc202/ingate/internal/controlplane/controller/trafficpolicy"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	externalversions "github.com/lgc202/ingate/pkg/generated/informers/externalversions"
)

const (
	defaultResyncPeriod = 30 * time.Second
	defaultQPS          = 20
	defaultBurst        = 40
)

type controllerRegistration interface {
	Name() string
	Register() error
}

func Run(ctx context.Context, out io.Writer, opts appoptions.CompletedOptions) error {
	if out == nil {
		out = io.Discard
	}

	cfg, err := controllerconfig.NewConfig(opts)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "starting %s\n", names.ControllerManagerName)
	fmt.Fprintf(out, "  apiserver-address: %s\n", cfg.Options.APIServerAddress)
	fmt.Fprintf(out, "  kubeconfig: %s\n", cfg.Options.Kubeconfig)
	fmt.Fprintf(out, "  leader-election: %t (%s/%s)\n", cfg.Options.LeaderElectionEnabled, cfg.Options.LeaderElectionNamespace, cfg.Options.LeaderElectionName)
	fmt.Fprintf(out, "  metrics-bind-address: %s\n", cfg.Options.MetricsBindAddress)
	fmt.Fprintf(out, "  healthz-bind-address: %s\n", cfg.Options.HealthzBindAddress)
	fmt.Fprintf(out, "  watch-namespace: %q\n", cfg.Options.WatchNamespace)
	fmt.Fprintf(out, "  workers: %d\n", cfg.Options.Workers)

	client, err := newClientset(cfg.Options)
	if err != nil {
		return err
	}

	informerFactory := newInformerFactory(client, cfg.Options)
	runtimeContext := controllerruntime.NewContext(client, informerFactory, controllerindex.New(), shared.NewGatewayQueue())
	resolvedGatewayController := resolvedgateway.NewController(runtimeContext)
	controllers := []controllerRegistration{
		controllergateway.NewController(runtimeContext),
		route.NewController(runtimeContext),
		backend.NewController(runtimeContext),
		certificate.NewController(runtimeContext),
		authpolicy.NewController(runtimeContext),
		trafficpolicy.NewController(runtimeContext),
	}
	for _, controller := range controllers {
		if err := controller.Register(); err != nil {
			return fmt.Errorf("register %s controller: %w", controller.Name(), err)
		}
		fmt.Fprintf(out, "  registered-controller: %s\n", controller.Name())
	}
	fmt.Fprintf(out, "  registered-controller: %s\n", names.ResolvedGatewayControllerName)

	go informerFactory.Start(ctx.Done())
	if !cache.WaitForCacheSync(
		ctx.Done(),
		informerFactory.Gateway().V1alpha1().Gateways().Informer().HasSynced,
		informerFactory.Gateway().V1alpha1().Routes().Informer().HasSynced,
		informerFactory.Gateway().V1alpha1().Backends().Informer().HasSynced,
		informerFactory.Gateway().V1alpha1().Certificates().Informer().HasSynced,
		informerFactory.Policy().V1alpha1().AuthPolicies().Informer().HasSynced,
		informerFactory.Policy().V1alpha1().TrafficPolicies().Informer().HasSynced,
	) {
		return fmt.Errorf("timed out waiting for controller-manager informer caches to sync")
	}
	go resolvedGatewayController.Run(ctx, cfg.Options.Workers)
	go func() {
		<-ctx.Done()
		runtimeContext.GatewayQueue.ShutDown()
	}()

	return cfg.HealthServer.Run(ctx)
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
			return nil, fmt.Errorf("build kubeconfig-based controller-manager rest config: %w", err)
		}
		cfg.UserAgent = names.ControllerManagerName
		cfg.QPS = defaultQPS
		cfg.Burst = defaultBurst
		return cfg, nil
	}

	return &rest.Config{
		Host:      opts.APIServerAddress,
		UserAgent: names.ControllerManagerName,
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

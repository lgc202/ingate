package config

import (
	"fmt"
	"strings"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	cliflag "k8s.io/component-base/cli/flag"

	"github.com/lgc202/ingate/cmd/controller-manager/names"
	controllerhealth "github.com/lgc202/ingate/internal/controlplane/controller/health"
)

const (
	defaultAPIServerAddress   = "https://127.0.0.1:18443"
	defaultKubeconfigPath     = ""
	defaultMetricsBindAddress = "127.0.0.1:18080"
	defaultHealthzBindAddress = "127.0.0.1:18081"
	defaultWatchNamespace     = ""
	defaultWorkers            = 1
)

type Options struct {
	APIServerAddress        string
	Kubeconfig              string
	LeaderElectionEnabled   bool
	LeaderElectionName      string
	LeaderElectionNamespace string
	MetricsBindAddress      string
	HealthzBindAddress      string
	WatchNamespace          string
	Workers                 int
}

type CompletedOptions struct {
	APIServerAddress        string
	Kubeconfig              string
	LeaderElectionEnabled   bool
	LeaderElectionName      string
	LeaderElectionNamespace string
	MetricsBindAddress      string
	HealthzBindAddress      string
	WatchNamespace          string
	Workers                 int
}

type Config struct {
	Options      CompletedOptions
	HealthServer *controllerhealth.Server
}

func NewOptions() *Options {
	return &Options{
		APIServerAddress:        defaultAPIServerAddress,
		Kubeconfig:              defaultKubeconfigPath,
		LeaderElectionEnabled:   false,
		LeaderElectionName:      names.ControllerManagerName,
		LeaderElectionNamespace: names.DefaultLeaderElectionNamespace,
		MetricsBindAddress:      defaultMetricsBindAddress,
		HealthzBindAddress:      defaultHealthzBindAddress,
		WatchNamespace:          defaultWatchNamespace,
		Workers:                 defaultWorkers,
	}
}

func (o *Options) AddFlags(fss *cliflag.NamedFlagSets) {
	if o == nil {
		return
	}

	general := fss.FlagSet("general")
	general.StringVar(&o.APIServerAddress, "apiserver-address", o.APIServerAddress, "The Ingate apiserver base URL.")
	general.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "Path to the kubeconfig file used to talk to the apiserver.")
	general.BoolVar(&o.LeaderElectionEnabled, "leader-elect", o.LeaderElectionEnabled, "Enable leader election for the controller manager.")
	general.StringVar(&o.LeaderElectionName, "leader-election-name", o.LeaderElectionName, "Leader election lease name.")
	general.StringVar(&o.LeaderElectionNamespace, "leader-election-namespace", o.LeaderElectionNamespace, "Namespace that stores leader election leases.")
	general.StringVar(&o.WatchNamespace, "watch-namespace", o.WatchNamespace, "Namespace to watch. Empty means all namespaces.")
	general.IntVar(&o.Workers, "workers", o.Workers, "Number of worker goroutines used by controllers.")

	metrics := fss.FlagSet("metrics")
	metrics.StringVar(&o.MetricsBindAddress, "metrics-bind-address", o.MetricsBindAddress, "The address the metrics endpoint binds to.")

	health := fss.FlagSet("health")
	health.StringVar(&o.HealthzBindAddress, "healthz-bind-address", o.HealthzBindAddress, "The address the health endpoints bind to.")
}

func (o *Options) Complete() (CompletedOptions, error) {
	if o == nil {
		o = NewOptions()
	}

	return CompletedOptions{
		APIServerAddress:        strings.TrimSpace(o.APIServerAddress),
		Kubeconfig:              strings.TrimSpace(o.Kubeconfig),
		LeaderElectionEnabled:   o.LeaderElectionEnabled,
		LeaderElectionName:      strings.TrimSpace(o.LeaderElectionName),
		LeaderElectionNamespace: strings.TrimSpace(o.LeaderElectionNamespace),
		MetricsBindAddress:      strings.TrimSpace(o.MetricsBindAddress),
		HealthzBindAddress:      strings.TrimSpace(o.HealthzBindAddress),
		WatchNamespace:          strings.TrimSpace(o.WatchNamespace),
		Workers:                 o.Workers,
	}, nil
}

func NewConfig(opts CompletedOptions) (*Config, error) {
	if errs := opts.Validate(); len(errs) != 0 {
		return nil, utilerrors.NewAggregate(errs)
	}

	healthServer, err := controllerhealth.NewServer(opts.HealthzBindAddress)
	if err != nil {
		return nil, err
	}

	return &Config{
		Options:      opts,
		HealthServer: healthServer,
	}, nil
}

func (o CompletedOptions) Validate() []error {
	var errs []error
	if o.APIServerAddress == "" {
		errs = append(errs, fmt.Errorf("apiserver-address must not be empty"))
	}
	if o.LeaderElectionEnabled {
		if o.LeaderElectionName == "" {
			errs = append(errs, fmt.Errorf("leader-election-name must not be empty when leader election is enabled"))
		}
		if o.LeaderElectionNamespace == "" {
			errs = append(errs, fmt.Errorf("leader-election-namespace must not be empty when leader election is enabled"))
		}
	}
	if o.MetricsBindAddress == "" {
		errs = append(errs, fmt.Errorf("metrics-bind-address must not be empty"))
	}
	if o.HealthzBindAddress == "" {
		errs = append(errs, fmt.Errorf("healthz-bind-address must not be empty"))
	}
	if o.Workers < 1 {
		errs = append(errs, fmt.Errorf("workers must be at least 1"))
	}
	return errs
}

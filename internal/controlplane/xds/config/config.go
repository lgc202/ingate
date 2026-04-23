package config

import (
	"fmt"
	"strings"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	cliflag "k8s.io/component-base/cli/flag"

	controllerhealth "github.com/lgc202/ingate/internal/controlplane/controller/health"
)

const (
	defaultAPIServerAddress = "https://127.0.0.1:18443"
	defaultKubeconfigPath   = ""
	defaultGRPCBindAddress  = "127.0.0.1:19090"
	defaultHealthzBindAddr  = "127.0.0.1:19091"
	defaultWatchNamespace   = ""
)

type Options struct {
	APIServerAddress   string
	Kubeconfig         string
	GRPCBindAddress    string
	HealthzBindAddress string
	WatchNamespace     string
}

type CompletedOptions struct {
	APIServerAddress   string
	Kubeconfig         string
	GRPCBindAddress    string
	HealthzBindAddress string
	WatchNamespace     string
}

type Config struct {
	Options      CompletedOptions
	HealthServer *controllerhealth.Server
}

func NewOptions() *Options {
	return &Options{
		APIServerAddress:   defaultAPIServerAddress,
		Kubeconfig:         defaultKubeconfigPath,
		GRPCBindAddress:    defaultGRPCBindAddress,
		HealthzBindAddress: defaultHealthzBindAddr,
		WatchNamespace:     defaultWatchNamespace,
	}
}

func (o *Options) AddFlags(fss *cliflag.NamedFlagSets) {
	if o == nil {
		return
	}

	general := fss.FlagSet("general")
	general.StringVar(&o.APIServerAddress, "apiserver-address", o.APIServerAddress, "The Ingate apiserver base URL.")
	general.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "Path to the kubeconfig file used to talk to the apiserver.")
	general.StringVar(&o.WatchNamespace, "watch-namespace", o.WatchNamespace, "Namespace to watch. Empty means all namespaces.")

	network := fss.FlagSet("network")
	network.StringVar(&o.GRPCBindAddress, "grpc-bind-address", o.GRPCBindAddress, "The address the xds/discovery gRPC server binds to.")

	health := fss.FlagSet("health")
	health.StringVar(&o.HealthzBindAddress, "healthz-bind-address", o.HealthzBindAddress, "The address the health endpoints bind to.")
}

func (o *Options) Complete() (CompletedOptions, error) {
	if o == nil {
		o = NewOptions()
	}

	return CompletedOptions{
		APIServerAddress:   strings.TrimSpace(o.APIServerAddress),
		Kubeconfig:         strings.TrimSpace(o.Kubeconfig),
		GRPCBindAddress:    strings.TrimSpace(o.GRPCBindAddress),
		HealthzBindAddress: strings.TrimSpace(o.HealthzBindAddress),
		WatchNamespace:     strings.TrimSpace(o.WatchNamespace),
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
	if o.GRPCBindAddress == "" {
		errs = append(errs, fmt.Errorf("grpc-bind-address must not be empty"))
	}
	if o.HealthzBindAddress == "" {
		errs = append(errs, fmt.Errorf("healthz-bind-address must not be empty"))
	}
	return errs
}

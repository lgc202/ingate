package options

import (
	cliflag "k8s.io/component-base/cli/flag"

	controllerconfig "github.com/lgc202/ingate/internal/controlplane/controller/config"
)

// ServerRunOptions owns the command-facing options for ingate-controller-manager.
type ServerRunOptions struct {
	*controllerconfig.Options
}

type CompletedOptions = controllerconfig.CompletedOptions

func NewServerRunOptions() *ServerRunOptions {
	return &ServerRunOptions{Options: controllerconfig.NewOptions()}
}

func (o *ServerRunOptions) Flags() cliflag.NamedFlagSets {
	var fss cliflag.NamedFlagSets
	o.Options.AddFlags(&fss)
	return fss
}

func (o *ServerRunOptions) Complete() (CompletedOptions, error) {
	return o.Options.Complete()
}

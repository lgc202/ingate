package options

import (
	cliflag "k8s.io/component-base/cli/flag"

	xdsconfig "github.com/lgc202/ingate/internal/controlplane/xds/config"
)

// ServerRunOptions owns the command-facing options for ingate-xds-server.
type ServerRunOptions struct {
	*xdsconfig.Options
}

type CompletedOptions = xdsconfig.CompletedOptions

func NewServerRunOptions() *ServerRunOptions {
	return &ServerRunOptions{Options: xdsconfig.NewOptions()}
}

func (o *ServerRunOptions) Flags() cliflag.NamedFlagSets {
	var fss cliflag.NamedFlagSets
	o.Options.AddFlags(&fss)
	return fss
}

func (o *ServerRunOptions) Complete() (CompletedOptions, error) {
	return o.Options.Complete()
}

package options

import (
	cliflag "k8s.io/component-base/cli/flag"

	controlplaneoptions "github.com/lgc202/ingate/internal/controlplane/apiserver/options"
)

// ServerRunOptions owns the command-facing options for ingate-apiserver.
type ServerRunOptions struct {
	*controlplaneoptions.Options
}

type CompletedOptions = controlplaneoptions.CompletedOptions

func NewServerRunOptions() *ServerRunOptions {
	return &ServerRunOptions{Options: controlplaneoptions.NewOptions()}
}

func (o *ServerRunOptions) Flags() cliflag.NamedFlagSets {
	var fss cliflag.NamedFlagSets
	o.Options.AddFlags(&fss)
	return fss
}

func (o *ServerRunOptions) Complete() (CompletedOptions, error) {
	return o.Options.Complete()
}

func (o *ServerRunOptions) Validate() []error {
	return o.Options.Validate()
}

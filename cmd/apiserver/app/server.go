package app

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"

	appoptions "github.com/lgc202/ingate/cmd/apiserver/app/options"
	controlplaneapiserver "github.com/lgc202/ingate/internal/controlplane/apiserver"
)

func NewAPIServerCommand() *cobra.Command {
	o := appoptions.NewServerRunOptions()

	cmd := &cobra.Command{
		Use:          "ingate-apiserver",
		Short:        "Launch the Ingate API server",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			completed, err := o.Complete()
			if err != nil {
				return err
			}
			if errs := o.Validate(); len(errs) != 0 {
				return utilerrors.NewAggregate(errs)
			}
			return Run(cmd.Context(), completed)
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

func Run(ctx context.Context, opts appoptions.CompletedOptions) error {
	fmt.Fprintf(os.Stdout, "starting ingate-apiserver on https://%s:%d\n", opts.GenericServerRunOptions.AdvertiseAddress.String(), opts.SecureServing.BindPort)

	config, err := controlplaneapiserver.NewConfig(opts)
	if err != nil {
		return err
	}

	server, err := config.Complete().New()
	if err != nil {
		return err
	}

	return server.GenericAPIServer.PrepareRun().RunWithContext(ctx)
}

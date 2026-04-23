package app

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	appoptions "github.com/lgc202/ingate/internal/adminapi/app/options"
	adminconfig "github.com/lgc202/ingate/internal/adminapi/config"
	adminserver "github.com/lgc202/ingate/internal/adminapi/server"
)

func NewAdminAPICommand() *cobra.Command {
	o := appoptions.NewOptions()

	cmd := &cobra.Command{
		Use:          "ingate-admin-api",
		Short:        "Launch the Ingate admin API server",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			completed, err := o.Complete()
			if err != nil {
				return err
			}
			if errs := completed.Validate(); len(errs) != 0 {
				return joinErrors(errs)
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

	o.AddFlags(cmd.Flags())
	return cmd
}

func Run(ctx context.Context, opts appoptions.CompletedOptions) error {
	cfg := adminconfig.New(opts)
	server, err := adminserver.New(cfg)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "starting ingate-admin-api on http://%s\n", cfg.ListenAddress())
	return server.Run(ctx)
}

func joinErrors(errs []error) error {
	if len(errs) == 1 {
		return errs[0]
	}
	return fmt.Errorf("validation failed: %v", errs)
}

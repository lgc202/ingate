package app

import (
	"fmt"

	"github.com/spf13/cobra"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"

	appoptions "github.com/lgc202/ingate/cmd/controller-manager/app/options"
	"github.com/lgc202/ingate/cmd/controller-manager/names"
)

func NewControllerManagerCommand() *cobra.Command {
	o := appoptions.NewServerRunOptions()

	cmd := &cobra.Command{
		Use:          names.ControllerManagerName,
		Short:        "Launch the Ingate controller manager",
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

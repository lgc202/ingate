package app

import "github.com/spf13/cobra"

const commandName = "ingatectl"

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          commandName,
		Short:        "Interact with local Ingate control-plane services",
		SilenceUsage: true,
	}

	cmd.AddCommand(newXDSCommand())
	return cmd
}

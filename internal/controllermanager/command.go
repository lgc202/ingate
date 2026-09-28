package controllermanager

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	mysqlstore "github.com/lgc202/ingate/internal/adminapi/repository/mysql"
)

// NewCommand 创建 Controller Manager 的根命令。
func NewCommand(ctx context.Context, options *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   componentName,
		Short: "Run the Ingate controller manager",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return options.Run(cmd.Context())
		},
	}
	cmd.SetContext(ctx)
	options.AddFlags(cmd.Flags())

	return cmd
}

// Run 组装并启动 Manager，直到 ctx 取消或运行失败。
func (o *Options) Run(ctx context.Context) error {
	bootstrap, err := loadConfig(o.ConfigFile)
	if err != nil {
		return err
	}

	store, err := mysqlstore.Open(bootstrap.GetMysql().GetDsn())
	if err != nil {
		return fmt.Errorf("open MySQL projection store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			slog.ErrorContext(ctx, "close MySQL projection store", "err", err)
		}
	}()

	config, err := o.Config()
	if err != nil {
		return err
	}

	manager, err := config.New(store)
	if err != nil {
		return err
	}

	if err := manager.Start(ctx); err != nil {
		return fmt.Errorf("run controller manager: %w", err)
	}

	return nil
}

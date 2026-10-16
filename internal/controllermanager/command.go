package controllermanager

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	mysqlstore "github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	mysqlclient "github.com/lgc202/ingate/internal/pkg/mysql"
	"github.com/lgc202/ingate/internal/pkg/version"
)

// NewCommand 创建 Controller Manager 的根命令。
func NewCommand(ctx context.Context, options *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     componentName,
		Short:   "Run the Ingate controller manager",
		Version: version.Get().Text(),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return options.Run(cmd.Context())
		},
	}
	cmd.SetContext(ctx)
	cmd.SetVersionTemplate("{{.Version}}\n")
	options.AddFlags(cmd.Flags())

	return cmd
}

// Run 组装并启动 Manager，直到 ctx 取消或运行失败。
func (o *Options) Run(ctx context.Context) (runErr error) {
	bootstrap, err := loadConfig(o.ConfigFile)
	if err != nil {
		return err
	}

	// 先完成 Flags、kubeconfig 和 Scheme 组装，再取得需要关闭的数据库连接池。
	config, err := o.Config()
	if err != nil {
		return err
	}

	db, err := mysqlclient.Open(bootstrap.GetMysql().GetDsn())
	if err != nil {
		return fmt.Errorf("open MySQL projection store: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get MySQL connection pool: %w", err)
	}

	defer func() {
		if err := sqlDB.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close MySQL projection store: %w", err))
		}
	}()
	store := mysqlstore.New(db)

	manager, err := config.New(store)
	if err != nil {
		return err
	}

	if err := manager.Start(ctx); err != nil {
		return fmt.Errorf("run controller manager: %w", err)
	}

	return nil
}

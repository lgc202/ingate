// Package app 实现 ingate-admin 命令入口
package app

import (
	"context"
	"io"

	"github.com/lgc202/go-kit/version"
	"github.com/spf13/cobra"

	"github.com/lgc202/ingate/internal/admin"
)

const commandDescription = `ingate-admin 提供 Ingate 管理 API

职责：
  - 聚合声明式资源，提供页面友好的接口
  - 通过 ingate-apiserver 写入网关期望状态
  - 管理客户端访问密钥并发布数据面认证索引`

// NewCommand 创建 ingate-admin 命令
func NewCommand() *cobra.Command {
	options := newServerOptions()
	command := &cobra.Command{
		Use:           "ingate-admin",
		Short:         "提供 Ingate 管理 API",
		Long:          commandDescription,
		Version:       version.Get().Text(),
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(command *cobra.Command, _ []string) error {
			return run(command.Context(), options.configFile, command.OutOrStdout())
		},
	}
	command.SetVersionTemplate("{{.Version}}\n")
	options.addFlags(command.Flags())
	return command
}

func run(ctx context.Context, configFile string, stdout io.Writer) error {
	loaded, config, err := loadConfig(configFile)
	if err != nil {
		return err
	}

	logger, err := config.Logging.NewLogger("ingate-admin", stdout)
	if err != nil {
		return err
	}
	defer logger.Close()
	watchConfig(loaded, logger)
	logger.Info("service started", "config_file", configFile)

	server, err := admin.New(ctx, config, logger.Logger)
	if err != nil {
		return err
	}
	return server.Run(ctx)
}

package apiserver

import (
	"context"

	"github.com/lgc202/ingate/internal/pkg/version"
	"github.com/spf13/cobra"
)

// NewCommand 创建 API Server 的启动命令。
func NewCommand(ctx context.Context, options *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     componentName,
		Short:   "Run the Ingate API server",
		Version: version.Get().Text(),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// 校验先于 Config：后者可能准备 HTTPS Listener。
			if err := options.Validate(); err != nil {
				return err
			}

			return options.Run(cmd.Context())
		},
	}
	cmd.SetContext(ctx)
	cmd.SetVersionTemplate("{{.Version}}\n")
	options.AddFlags(cmd.Flags())

	return cmd
}

// Run 组装并运行 API Server，直到 ctx 取消或服务返回错误。
func (o *Options) Run(ctx context.Context) error {
	config, err := o.Config()
	if err != nil {
		return err
	}

	server, err := config.Complete().New()
	if err != nil {
		return err
	}

	// Generic API Server 统一管理启动与优雅退出。
	return server.GenericAPIServer.PrepareRun().RunWithContext(ctx)
}

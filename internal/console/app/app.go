// Package app 实现 ingate-console 服务入口
package app

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	kitconfig "github.com/lgc202/go-kit/config"
	"github.com/lgc202/go-kit/version"
	"github.com/spf13/pflag"

	consoleserver "github.com/lgc202/ingate/internal/console/server"
	"github.com/lgc202/ingate/internal/pkg/httpserver"
)

const usage = `ingate-console 提供 Ingate 控制台

职责：
  - 托管控制台静态资源
  - 将控制台管理请求转发给 ingate-admin-api
`

// Run 执行 ingate-console 服务
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := pflag.NewFlagSet("ingate-console", pflag.ContinueOnError)
	flags.SetOutput(stderr)

	configPath := defaultConfigPath
	showVersion := false
	flags.StringVar(&configPath, "config", configPath, "配置文件路径")
	flags.BoolVar(&showVersion, "version", showVersion, "显示版本信息")
	flags.Usage = func() {
		fmt.Fprint(stdout, usage)
		fmt.Fprintln(stdout)
		flags.SetOutput(stdout)
		flags.PrintDefaults()
		flags.SetOutput(stderr)
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil
		}
		return err
	}
	if showVersion {
		fmt.Fprintln(stdout, version.Get().Text())
		return nil
	}

	loaded, err := kitconfig.Load[Config](configPath, kitconfig.WithEnv[Config]("INGATE_CONSOLE"))
	if err != nil {
		return err
	}
	settings := loaded.Get()
	if err := settings.Validate(); err != nil {
		return err
	}

	logger, err := settings.Logging.NewLogger("ingate-console", stdout)
	if err != nil {
		return err
	}
	defer logger.Close()
	loaded.OnChange(func(old, next Config) {
		if err := next.Validate(); err != nil {
			logger.Error("ignoring invalid configuration change", "err", err)
			return
		}
		old.Logging.ApplyDynamic(next.Logging, logger)
		old.Logging = next.Logging
		if kitconfig.Changed(old, next) {
			logger.Warn("service configuration changed and requires a restart")
		}
	})
	logger.Info("service started", "config_file", configPath)

	componentLogger := logger.With("component", "ingate-console")
	adminAPIProxy, err := consoleserver.NewAdminAPIProxy(settings.AdminAPI.BaseURL, componentLogger)
	if err != nil {
		return err
	}

	gin.SetMode(gin.ReleaseMode)
	router := consoleserver.NewRouter(adminAPIProxy, settings.Server.ConsoleDir, componentLogger)
	server := httpserver.New(settings.Server.ListenAddress, router, componentLogger)

	return server.Run(ctx)
}

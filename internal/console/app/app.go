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
	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/console/handler"
	consoleserver "github.com/lgc202/ingate/internal/console/server"
	"github.com/lgc202/ingate/internal/console/service"
	"github.com/lgc202/ingate/internal/console/store"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

const usage = `ingate-console 是面向前端控制台的管理 API

职责：
  - 聚合声明式资源，提供页面友好的接口
  - 处理登录、租户、权限和审计等管理侧能力
  - 通过 ingate-apiserver 写入网关期望状态
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

	restConfig, err := clientcmd.BuildConfigFromFlags(settings.APIServer.Master, settings.APIServer.Kubeconfig)
	if err != nil {
		return fmt.Errorf("build apiserver client config: %w", err)
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("create apiserver resource client: %w", err)
	}

	componentLogger := logger.With("component", "ingate-console")
	stores := store.New(client)
	services := service.New(stores)
	handlers := handler.New(services, componentLogger)

	gin.SetMode(gin.ReleaseMode)
	router := consoleserver.NewRouter(handlers, settings.Server.ConsoleDir, componentLogger)
	server := consoleserver.New(
		settings.Server.ListenAddress,
		router,
		componentLogger,
	)

	return server.Run(ctx)
}

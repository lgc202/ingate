// Package app 实现 ingate-admin-api 服务入口
package app

import (
	"errors"
	"fmt"
	"io"

	kitconfig "github.com/lgc202/go-kit/config"
	"github.com/lgc202/go-kit/version"
	"github.com/spf13/pflag"
	genericserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/tools/clientcmd"

	adminserver "github.com/lgc202/ingate/internal/adminapi/server"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

const usage = `ingate-admin-api 是面向前端控制台的管理 API

职责：
  - 聚合声明式资源，提供页面友好的接口
  - 处理登录、租户、权限和审计等管理侧能力
  - 通过 ingate-apiserver 写入网关期望状态
`

// Run 执行 ingate-admin-api 服务
func Run(args []string, stdout, stderr io.Writer) error {
	flags := pflag.NewFlagSet("ingate-admin-api", pflag.ContinueOnError)
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

	loaded, err := kitconfig.Load[Config](configPath, kitconfig.WithEnv[Config]("INGATE_ADMIN_API"))
	if err != nil {
		return err
	}
	settings := loaded.Get()
	if err := settings.Validate(); err != nil {
		return err
	}

	logger, err := settings.Logging.NewLogger("ingate-admin-api", stdout)
	if err != nil {
		return err
	}
	defer logger.Close()
	loaded.OnChange(func(old, next Config) {
		if err := next.Validate(); err != nil {
			logger.Error("忽略无效的配置文件变更", "err", err)
			return
		}
		old.Logging.ApplyDynamic(next.Logging, logger)
		old.Logging = next.Logging
		if kitconfig.Changed(old, next) {
			logger.Warn("服务配置已变化，将在重启后生效")
		}
	})
	logger.Info("服务启动", "config_file", configPath)

	restConfig, err := clientcmd.BuildConfigFromFlags(settings.APIServer.Master, settings.APIServer.Kubeconfig)
	if err != nil {
		return err
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	server := adminserver.New(
		client,
		settings.Server.ListenAddress,
		settings.Server.ConsoleDir,
		logger.With("component", "ingate-admin-api"),
	)

	return server.Run(genericserver.SetupSignalContext())
}

// Package app 实现 ingate-apiserver 服务入口
package app

import (
	"errors"
	"fmt"
	"io"

	kitconfig "github.com/lgc202/go-kit/config"
	"github.com/lgc202/go-kit/version"
	"github.com/spf13/pflag"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"

	"github.com/lgc202/ingate/internal/apiserver/server"
)

const usage = `ingate-apiserver 是声明式资源 API

职责：
  - 接收 Gateway、Route、Upstream、Policy 等资源的 apply/get/list/watch
  - 校验资源并维护期望状态
  - 作为 CLI、ingate-admin-api 和 ingate-controller 的统一控制面入口
`

// Run 执行 ingate-apiserver 服务
func Run(args []string, stdout, stderr io.Writer) error {
	flags := pflag.NewFlagSet("ingate-apiserver", pflag.ContinueOnError)
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

	loaded, err := kitconfig.Load[Config](configPath, kitconfig.WithEnv[Config]("INGATE_APISERVER"))
	if err != nil {
		return err
	}
	settings := loaded.Get()
	if err := settings.Validate(); err != nil {
		return err
	}

	logger, err := settings.Logging.NewLogger("ingate-apiserver", stdout)
	if err != nil {
		return err
	}
	defer logger.Close()
	klog.SetSlogLogger(logger.Logger)
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

	options := server.NewOptions(stdout, stderr)
	options.SecureServing.BindAddress = netutils.ParseIPSloppy(settings.Server.BindAddress)
	options.SecureServing.BindPort = settings.Server.SecurePort
	options.SecureServing.ServerCert.CertDirectory = settings.Server.CertDirectory
	options.Etcd.StorageConfig.Transport.ServerList = append([]string(nil), settings.Etcd.Endpoints...)
	options.Etcd.StorageConfig.Prefix = settings.Etcd.Prefix
	if err := options.Complete(); err != nil {
		return err
	}
	if err := options.Validate(); err != nil {
		return err
	}

	serverConfig, err := options.Config()
	if err != nil {
		return err
	}
	apiServer, err := serverConfig.Complete().New(genericapiserver.NewEmptyDelegate())
	if err != nil {
		return err
	}

	return apiServer.Run(genericapiserver.SetupSignalContext())
}

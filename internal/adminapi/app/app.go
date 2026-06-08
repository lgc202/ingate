// Package app 实现 ingate-admin-api 服务入口
package app

import (
	"fmt"
	"io"

	"github.com/spf13/pflag"
	genericserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/tools/clientcmd"

	adminserver "github.com/lgc202/ingate/internal/adminapi/server"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	"github.com/lgc202/ingate/pkg/xlog"
)

const usage = `ingate-admin-api 是面向前端控制台的管理 API

职责：
  - 聚合声明式资源、运行状态和统计数据，提供页面友好的接口
  - 处理登录、租户、权限和审计等管理侧能力
  - 通过 ingate-apiserver 写入网关期望状态
`

// Run 执行 ingate-admin-api 服务
func Run(args []string, stdout, stderr io.Writer) error {
	flags := pflag.NewFlagSet("ingate-admin-api", pflag.ContinueOnError)
	flags.SetOutput(stderr)

	options := NewOptions()
	options.AddFlags(flags)
	flags.Usage = func() {
		fmt.Fprint(stdout, usage)
		fmt.Fprintln(stdout)
		flags.SetOutput(stdout)
		flags.PrintDefaults()
		flags.SetOutput(stderr)
	}
	if err := flags.Parse(args); err != nil {
		if err == pflag.ErrHelp {
			return nil
		}
		return err
	}

	config, err := clientcmd.BuildConfigFromFlags(options.Master, options.Kubeconfig)
	if err != nil {
		return err
	}
	client, err := clientset.NewForConfig(config)
	if err != nil {
		return err
	}
	logger, err := xlog.New(xlog.Options{
		Output: logOutput(options.LogStdout, stdout),
		Format: options.LogFormat,
		Level:  options.LogLevel,
		File:   options.LogFile,
	})
	if err != nil {
		return err
	}
	defer logger.Close()

	server := adminserver.New(client, options.ListenAddress, options.ConsoleDir, logger.With("component", "ingate-admin-api"))

	return server.Run(genericserver.SetupSignalContext())
}

func logOutput(enabled bool, stdout io.Writer) io.Writer {
	if !enabled {
		return nil
	}
	return stdout
}

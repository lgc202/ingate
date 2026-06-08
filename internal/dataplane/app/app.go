// Package app 实现 ingate-dataplane 服务入口
package app

import (
	"fmt"
	"io"

	"github.com/spf13/pflag"
	genericserver "k8s.io/apiserver/pkg/server"

	dataplaneserver "github.com/lgc202/ingate/internal/dataplane/server"
	"github.com/lgc202/ingate/pkg/xlog"
)

const usage = `ingate-dataplane 是和数据面同生命周期运行的能力服务

职责：
  - 承载 Wasm 插件不适合直接完成的外部调用能力
  - 为内置治理插件提供稳定的 capability API
  - 统一管理 Redis、Kafka 等外部系统连接、观测和故障处理
`

// Run 执行 ingate-dataplane 服务
func Run(args []string, stdout, stderr io.Writer) error {
	flags := pflag.NewFlagSet("ingate-dataplane", pflag.ContinueOnError)
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

	server := dataplaneserver.New(options.ListenAddress, logger.With("component", "ingate-dataplane"))
	return server.Run(genericserver.SetupSignalContext())
}

func logOutput(enabled bool, stdout io.Writer) io.Writer {
	if !enabled {
		return nil
	}
	return stdout
}

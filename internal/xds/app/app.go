// Package app 实现 ingate-xds 服务入口
package app

import (
	"fmt"
	"io"

	"github.com/spf13/pflag"
	genericserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/tools/clientcmd"

	xdsserver "github.com/lgc202/ingate-next/internal/xds/server"
	clientset "github.com/lgc202/ingate-next/pkg/generated/clientset/versioned"
)

const usage = `ingate-xds 是面向 Envoy 的 xDS 配置服务

职责：
  - 基于 RuntimeSnapshot 对 Envoy 提供 xDS 配置
  - 维护数据面连接状态
  - 记录 Envoy ACK/NACK，辅助定位配置下发问题
`

// Run 执行 ingate-xds 服务
func Run(args []string, stdout, stderr io.Writer) error {
	flags := pflag.NewFlagSet("ingate-xds", pflag.ContinueOnError)
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
	xdsServer := xdsserver.New(client, options.ListenAddress, options.Target, options.ResyncPeriod, stdout)

	return xdsServer.Run(genericserver.SetupSignalContext())
}

// Package app 实现 ingate-apiserver 服务入口
package app

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/pflag"
	genericapiserver "k8s.io/apiserver/pkg/server"

	"github.com/lgc202/ingate/internal/apiserver/server"
)

const usage = `ingate-apiserver 是声明式资源 API

职责：
  - 接收 Gateway、Route、Upstream、Policy 等资源的 apply/get/list/watch
  - 校验资源并维护期望状态
  - 作为 CLI、admin-api 和 controller 的统一控制面入口
`

// Run 执行 ingate-apiserver 服务
func Run(args []string, stdout, stderr io.Writer) error {
	flags := pflag.NewFlagSet("ingate-apiserver", pflag.ContinueOnError)
	flags.SetOutput(stderr)
	options := server.NewOptions(stdout, stderr)
	options.AddFlags(flags)
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
	if err := options.Complete(); err != nil {
		return err
	}
	if err := options.Validate(); err != nil {
		return err
	}

	config, err := options.Config()
	if err != nil {
		return err
	}
	apiServer, err := config.Complete().New(genericapiserver.NewEmptyDelegate())
	if err != nil {
		return err
	}

	return apiServer.Run(genericapiserver.SetupSignalContext())
}

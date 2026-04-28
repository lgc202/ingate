// Package app 实现 ingate-apiserver 服务入口
package app

import (
	"flag"
	"fmt"
	"io"
)

const usage = `ingate-apiserver 是声明式资源 API

职责：
  - 接收 Gateway、Route、Upstream、Policy 等资源的 apply/get/list/watch
  - 校验资源并维护期望状态
  - 作为 CLI、admin-api、controller 和 operator 的统一控制面入口
`

// Run 执行 ingate-apiserver 服务
func Run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("ingate-apiserver", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(stdout, usage)
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	return fmt.Errorf("ingate-apiserver runtime is not implemented yet")
}

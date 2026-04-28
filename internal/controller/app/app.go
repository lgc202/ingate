// Package app 实现 ingate-controller 服务入口
package app

import (
	"flag"
	"fmt"
	"io"
)

const usage = `ingate-controller 负责声明式资源的状态收敛

职责：
  - 监听 ingate-apiserver 中的资源变化
  - 调用 compiler 和 pipeline 生成 RuntimeSnapshot
  - 推进资源状态并为 ingate-xds 准备运行时配置
`

// Run 执行 ingate-controller 服务
func Run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("ingate-controller", flag.ContinueOnError)
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

	return fmt.Errorf("ingate-controller runtime is not implemented yet")
}

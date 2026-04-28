// Package app 实现 ingate-xds 服务入口
package app

import (
	"flag"
	"fmt"
	"io"
)

const usage = `ingate-xds 是面向 Envoy 的 xDS 配置服务

职责：
  - 基于 RuntimeSnapshot 对 Envoy 提供 xDS 配置
  - 维护数据面连接状态
  - 记录 Envoy ACK/NACK，辅助定位配置下发问题
`

// Run 执行 ingate-xds 服务
func Run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("ingate-xds", flag.ContinueOnError)
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

	return fmt.Errorf("ingate-xds runtime is not implemented yet")
}

// Package app 实现 ingate-admin-api 服务入口
package app

import (
	"flag"
	"fmt"
	"io"
)

const usage = `ingate-admin-api 是面向前端控制台的管理 API

职责：
  - 聚合声明式资源、运行状态和统计数据，提供页面友好的接口
  - 处理登录、租户、权限和审计等管理侧能力
  - 通过 ingate-apiserver 写入网关期望状态
`

// Run 执行 ingate-admin-api 服务
func Run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("ingate-admin-api", flag.ContinueOnError)
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

	return fmt.Errorf("ingate-admin-api runtime is not implemented yet")
}

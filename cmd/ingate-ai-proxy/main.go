// Command ingate-ai-proxy 运行 Ingate AI Proxy，不提供管理或迁移命令。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/lgc202/ingate/internal/aiproxy"
	"github.com/lgc202/ingate/internal/pkg/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("ingate-ai-proxy", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configFile := flags.String("conf", "configs/ai-proxy.yaml", "AI Proxy configuration file")
	showVersion := flags.Bool("version", false, "Print build information and exit")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *showVersion {
		if _, err := fmt.Fprintln(stdout, version.Get().Text()); err != nil {
			// 最终诊断没有其他通道；输出失败保留退出码 1。
			_, _ = fmt.Fprintf(stderr, "ingate-ai-proxy: print version: %v\n", err)
			return 1
		}
		return 0
	}

	if err := aiproxy.Run(context.Background(), *configFile); err != nil {
		// 最终诊断没有其他通道；不在组件内部重复打印运行错误。
		_, _ = fmt.Fprintf(stderr, "ingate-ai-proxy: %v\n", err)
		return 1
	}

	return 0
}

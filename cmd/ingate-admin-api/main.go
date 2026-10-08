// Command ingate-admin-api 运行 Ingate Admin API。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/lgc202/ingate/internal/adminapi"
	"github.com/lgc202/ingate/internal/pkg/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("ingate-admin-api", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configFile := flags.String("conf", "configs/admin-api.yaml", "Admin API configuration file")
	migrate := flags.Bool("migrate", false, "Apply pending database migrations and exit")
	showVersion := flags.Bool("version", false, "Print build information and exit")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *showVersion {
		if _, err := fmt.Fprintln(stdout, version.Get().Text()); err != nil {
			// 最终诊断没有其他输出通道；写入失败也保留退出码 1。
			_, _ = fmt.Fprintf(stderr, "ingate-admin-api: print version: %v\n", err)
			return 1
		}
		return 0
	}

	ctx := context.Background()
	var err error
	if *migrate {
		err = adminapi.Migrate(ctx, *configFile)
	} else {
		err = adminapi.Run(ctx, *configFile)
	}
	if err != nil {
		// 最终诊断没有其他输出通道；写入失败也保留退出码 1。
		_, _ = fmt.Fprintf(stderr, "ingate-admin-api: %v\n", err)
		return 1
	}
	return 0
}

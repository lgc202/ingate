// Command ingate-assistant 启动运维助手的 HTTP 服务与 Temporal Worker。
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lgc202/ingate/internal/assistant"
	"github.com/lgc202/ingate/internal/pkg/version"

	_ "go.uber.org/automaxprocs"
)

const defaultConfigFile = "configs/ingate-assistant.yaml"

const roleAll = "all"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	configFile := flag.String("config", defaultConfigFile, "configuration file")
	role := flag.String("role", roleAll, "process role (all)")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		_, err := fmt.Fprintln(os.Stdout, version.Text())
		return err
	}
	if *role != roleAll {
		return fmt.Errorf("unsupported Assistant role %q; only %q is available", *role, roleAll)
	}
	app, err := assistant.NewApp(*configFile)
	if err != nil {
		return err
	}
	return app.Run()
}

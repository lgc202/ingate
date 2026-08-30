// Command ingate-admin-api 提供控制台使用的网关管理 HTTP API。
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lgc202/ingate/internal/adminapi"
	"github.com/lgc202/ingate/internal/pkg/version"

	_ "go.uber.org/automaxprocs"
)

const defaultConfigFile = "configs/ingate-admin-api.yaml"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	configFile := flag.String("config", defaultConfigFile, "configuration file")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		_, err := fmt.Fprintln(os.Stdout, version.Text())
		return err
	}
	app, err := adminapi.NewApp(*configFile)
	if err != nil {
		return err
	}
	return app.Run()
}

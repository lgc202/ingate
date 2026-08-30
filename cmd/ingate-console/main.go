// Command ingate-console 提供 Console 静态资源和管理面反向代理。
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lgc202/ingate/internal/console"
	"github.com/lgc202/ingate/internal/pkg/version"

	_ "go.uber.org/automaxprocs"
)

const defaultConfigFile = "configs/ingate-console.yaml"

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
	app, err := console.NewApp(*configFile)
	if err != nil {
		return err
	}
	return app.Run()
}

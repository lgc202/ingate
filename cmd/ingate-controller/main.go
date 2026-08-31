// Command ingate-controller 将声明式资源编译并发布为 Envoy xDS 配置。
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lgc202/ingate/internal/controller"
	"github.com/lgc202/ingate/internal/pkg/version"

	_ "go.uber.org/automaxprocs"
)

const defaultConfigFile = "configs/ingate-controller.yaml"

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
	app, err := controller.NewApp(*configFile)
	if err != nil {
		return err
	}
	return app.Run()
}

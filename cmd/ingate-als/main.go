// Command ingate-als 接收 Envoy 请求记录，并可靠投递到 Kafka。
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lgc202/ingate/internal/als"
	"github.com/lgc202/ingate/internal/pkg/version"

	_ "go.uber.org/automaxprocs"
)

const defaultConfigFile = "configs/ingate-als.yaml"

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
		if _, err := fmt.Fprintln(os.Stdout, version.Text()); err != nil {
			return fmt.Errorf("print version: %w", err)
		}
		return nil
	}

	app, err := als.NewApp(*configFile)
	if err != nil {
		return fmt.Errorf("create ALS application: %w", err)
	}
	if err := app.Run(); err != nil {
		return fmt.Errorf("run ALS application: %w", err)
	}
	return nil
}

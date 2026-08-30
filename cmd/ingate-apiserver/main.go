// Command ingate-apiserver 提供声明式网关资源 API。
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lgc202/ingate/internal/apiserver"
	"github.com/lgc202/ingate/internal/pkg/version"

	_ "go.uber.org/automaxprocs"
)

const defaultConfigFile = "configs/ingate-apiserver.yaml"

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
	app, err := apiserver.NewApp(*configFile)
	if err != nil {
		return err
	}
	return app.Run()
}

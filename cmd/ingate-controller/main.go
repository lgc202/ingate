// ingate-controller 将声明式资源编译并发布为 Envoy xDS 配置
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
	configFile := flag.String("config", defaultConfigFile, "configuration file")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.Text())
		return
	}
	app, err := controller.NewApp(*configFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

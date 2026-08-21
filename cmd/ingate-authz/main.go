// ingate-authz 为 Envoy 提供 Caller 访问密钥与 Route 权限校验
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lgc202/ingate/internal/authz"
	"github.com/lgc202/ingate/internal/pkg/version"

	_ "go.uber.org/automaxprocs"
)

const defaultConfigFile = "configs/ingate-authz.yaml"

func main() {
	configFile := flag.String("config", defaultConfigFile, "configuration file")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.Text())
		return
	}
	app, err := authz.NewApp(*configFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

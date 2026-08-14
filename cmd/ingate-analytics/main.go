// ingate-analytics 将 ALS 请求记录持久化到 ClickHouse
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lgc202/go-kit/version"

	"github.com/lgc202/ingate/internal/analytics"

	_ "go.uber.org/automaxprocs"
)

const defaultConfigFile = "configs/ingate-analytics.yaml"

func main() {
	configFile := flag.String("config", defaultConfigFile, "configuration file")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.Get().Text())
		return
	}
	app, err := analytics.NewApp(*configFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

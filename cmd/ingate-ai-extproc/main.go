// ingate-ai-extproc 为 Envoy 提供 AI 请求和响应的 External Processing 服务
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lgc202/go-kit/version"

	"github.com/lgc202/ingate/internal/aiextproc"

	_ "go.uber.org/automaxprocs"
)

const defaultConfigFile = "configs/ingate-ai-extproc.yaml"

func main() {
	configFile := flag.String("config", defaultConfigFile, "configuration file")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.Get().Text())
		return
	}
	app, err := aiextproc.NewApp(*configFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

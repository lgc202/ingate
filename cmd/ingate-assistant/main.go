// ingate-assistant 为控制台提供会话、模型执行与流式结果服务。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/lgc202/ingate/internal/assistant"
	"github.com/lgc202/ingate/internal/pkg/version"

	_ "go.uber.org/automaxprocs"
)

const defaultConfigFile = "configs/ingate-assistant.yaml"

func main() {
	configFile := flag.String("config", defaultConfigFile, "configuration file")
	migrateSchema := flag.Bool("migrate", false, "apply MySQL schema migrations and exit")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.Text())
		return
	}
	if *migrateSchema {
		applied, err := assistant.Migrate(context.Background(), *configFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("applied %d MySQL migration(s)\n", applied)
		return
	}
	app, err := assistant.NewApp(*configFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

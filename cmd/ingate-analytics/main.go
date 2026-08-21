// ingate-analytics 消费 ALS 请求记录，写入 ClickHouse，并提供分析查询服务
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/lgc202/ingate/internal/analytics"
	"github.com/lgc202/ingate/internal/pkg/version"

	_ "go.uber.org/automaxprocs"
)

const defaultConfigFile = "configs/ingate-analytics.yaml"

func main() {
	configFile := flag.String("config", defaultConfigFile, "configuration file")
	migrateSchema := flag.Bool("migrate", false, "apply ClickHouse schema migrations and exit")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.Text())
		return
	}
	if *migrateSchema {
		applied, err := analytics.Migrate(context.Background(), *configFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("applied %d ClickHouse migration(s)\n", applied)
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

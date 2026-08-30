// Command ingate-analytics 消费 ALS 请求记录，写入 ClickHouse，并提供分析查询服务。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lgc202/ingate/internal/analytics"
	"github.com/lgc202/ingate/internal/pkg/version"

	_ "go.uber.org/automaxprocs"
)

const defaultConfigFile = "configs/ingate-analytics.yaml"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	configFile := flag.String("config", defaultConfigFile, "configuration file")
	migrateSchema := flag.Bool("migrate", false, "apply ClickHouse schema migrations and exit")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		_, err := fmt.Fprintln(os.Stdout, version.Text())
		return err
	}
	if *migrateSchema {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		applied, err := analytics.Migrate(ctx, *configFile)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(os.Stdout, "applied %d ClickHouse migration(s)\n", applied)
		return err
	}
	app, err := analytics.NewApp(*configFile)
	if err != nil {
		return err
	}
	return app.Run()
}

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lgc202/go-kit/version"

	"github.com/lgc202/ingate/internal/console"

	_ "go.uber.org/automaxprocs"
)

const defaultConfigFile = "configs/ingate-console.yaml"

func main() {
	configFile := flag.String("config", defaultConfigFile, "configuration file")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.Get().Text())
		return
	}
	app, err := console.NewApp(*configFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

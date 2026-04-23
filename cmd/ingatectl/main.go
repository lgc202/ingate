package main

import (
	"fmt"
	"os"

	"github.com/lgc202/ingate/cmd/ingatectl/app"
	"github.com/lgc202/ingate/pkg/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version.Get().String())
		return
	}

	command := app.NewCommand()
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}

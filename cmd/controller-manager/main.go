package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lgc202/ingate/cmd/controller-manager/app"
	"github.com/lgc202/ingate/pkg/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version.Get().String())
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	command := app.NewControllerManagerCommand()
	command.SetContext(ctx)
	if err := command.Execute(); err != nil && !errors.Is(err, context.Canceled) {
		os.Exit(1)
	}
}

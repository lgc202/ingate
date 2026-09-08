// Command als-e2e provides the process probes used by the local ALS fault drill.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("command is required")
	}

	switch args[0] {
	case "serve":
		return serve(ctx, args[1:])
	case "send":
		return send(ctx, args[1:])
	case "ready":
		return waitReady(ctx, args[1:])
	case "http":
		return waitHTTP(ctx, args[1:])
	case "kafka":
		return waitKafka(ctx, args[1:])
	case "duplicate":
		return verifyDuplicate(ctx, args[1:])
	case "corrupt":
		return copyCorruptQueue(args[1:])
	case "observe":
		return verifyObservability(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

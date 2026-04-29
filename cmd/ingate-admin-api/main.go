package main

import (
	"fmt"
	"os"

	"github.com/lgc202/ingate/internal/adminapi/app"
)

func main() {
	if err := app.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

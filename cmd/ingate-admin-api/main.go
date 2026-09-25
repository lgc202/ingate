// Command ingate-admin-api 运行 Ingate Admin API。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/lgc202/ingate/internal/adminapi"
)

func main() {
	configFile := flag.String("conf", "configs/admin-api.yaml", "Admin API configuration file")
	flag.Parse()

	if err := adminapi.Run(context.Background(), *configFile); err != nil {
		fmt.Fprintf(os.Stderr, "ingate-admin-api: %v\n", err)
		os.Exit(1)
	}
}

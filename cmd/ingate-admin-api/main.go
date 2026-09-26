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
	migrate := flag.Bool("migrate", false, "Apply pending database migrations and exit")
	flag.Parse()

	ctx := context.Background()
	var err error
	if *migrate {
		err = adminapi.Migrate(ctx, *configFile)
	} else {
		err = adminapi.Run(ctx, *configFile)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "ingate-admin-api: %v\n", err)
		os.Exit(1)
	}
}

// Command ingate-apiserver 运行 Ingate Generic API Server。
package main

import (
	"fmt"
	"os"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/cli"

	"github.com/lgc202/ingate/internal/apiserver"
)

func main() {
	options, err := apiserver.NewOptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "create API server options: %v\n", err)
		os.Exit(1)
	}

	ctx := genericapiserver.SetupSignalContext()
	cmd := apiserver.NewCommand(ctx, options)
	os.Exit(cli.Run(cmd))
}

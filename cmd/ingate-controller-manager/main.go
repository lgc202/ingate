// Command ingate-controller-manager 运行 Ingate 共享数据面 Controller Manager。
package main

import (
	"os"

	"k8s.io/component-base/cli"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/lgc202/ingate/internal/controllermanager"
)

func main() {
	ctrl.SetLogger(klog.Background())
	cmd := controllermanager.NewCommand(ctrl.SetupSignalHandler(), controllermanager.NewOptions())
	os.Exit(cli.Run(cmd))
}

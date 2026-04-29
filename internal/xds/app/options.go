package app

import (
	"time"

	"github.com/spf13/pflag"
)

const (
	defaultListenAddress = ":18000"
	defaultTarget        = "xds"
	defaultResyncPeriod  = 30 * time.Second
)

// Options 表示 ingate-xds 启动参数
type Options struct {
	ListenAddress string
	Master        string
	Kubeconfig    string
	Target        string
	ResyncPeriod  time.Duration
}

// NewOptions 创建 ingate-xds 默认启动参数
func NewOptions() *Options {
	return &Options{
		ListenAddress: defaultListenAddress,
		Target:        defaultTarget,
		ResyncPeriod:  defaultResyncPeriod,
	}
}

// AddFlags 注册 ingate-xds 命令行参数
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.ListenAddress, "listen-address", o.ListenAddress, "xDS gRPC 监听地址")
	flags.StringVar(&o.Master, "master", o.Master, "ingate-apiserver 地址")
	flags.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "连接 ingate-apiserver 的 kubeconfig 路径")
	flags.StringVar(&o.Target, "target", o.Target, "消费的 RuntimeSnapshot target")
	flags.DurationVar(&o.ResyncPeriod, "resync-period", o.ResyncPeriod, "informer 全量 resync 周期")
}

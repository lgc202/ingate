package app

import "github.com/spf13/pflag"

const defaultListenAddress = ":8081"

// Options 表示 ingate-admin-api 启动参数
type Options struct {
	ListenAddress string
	Master        string
	Kubeconfig    string
}

// NewOptions 创建 ingate-admin-api 默认启动参数
func NewOptions() *Options {
	return &Options{ListenAddress: defaultListenAddress}
}

// AddFlags 注册 ingate-admin-api 命令行参数
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.ListenAddress, "listen-address", o.ListenAddress, "管理 API HTTP 监听地址")
	flags.StringVar(&o.Master, "master", o.Master, "ingate-apiserver 地址")
	flags.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "连接 ingate-apiserver 的 kubeconfig 路径")
}

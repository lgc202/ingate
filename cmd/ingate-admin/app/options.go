package app

import "github.com/spf13/pflag"

const defaultConfigFile = "configs/ingate-admin.yaml"

type serverOptions struct {
	configFile string
}

func newServerOptions() *serverOptions {
	return &serverOptions{configFile: defaultConfigFile}
}

func (o *serverOptions) addFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.configFile, "config", o.configFile, "配置文件路径")
}

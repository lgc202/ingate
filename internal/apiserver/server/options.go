package server

import (
	"io"

	"github.com/spf13/pflag"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"

	gatewayv1 "github.com/lgc202/ingate-next/pkg/apis/gateway/v1"
)

const (
	// DefaultEtcdPathPrefix 表示 ingate-apiserver 在 etcd 中使用的默认前缀
	DefaultEtcdPathPrefix = "/registry/gateway.ingate.io"
)

// Options 表示 ingate-apiserver 启动参数
type Options struct {
	ServerRunOptions   *genericoptions.ServerRunOptions
	RecommendedOptions *genericoptions.RecommendedOptions

	StdOut io.Writer
	StdErr io.Writer
}

// NewOptions 创建 ingate-apiserver 默认启动参数
func NewOptions(stdout, stderr io.Writer) *Options {
	return &Options{
		ServerRunOptions: genericoptions.NewServerRunOptions(),
		RecommendedOptions: genericoptions.NewRecommendedOptions(
			DefaultEtcdPathPrefix,
			Codecs.LegacyCodec(gatewayv1.SchemeGroupVersion),
		),
		StdOut: stdout,
		StdErr: stderr,
	}
}

// AddFlags 注册 ingate-apiserver 命令行参数
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	o.ServerRunOptions.AddUniversalFlags(flags)
	o.RecommendedOptions.AddFlags(flags)
}

// Validate 校验 ingate-apiserver 启动参数
func (o *Options) Validate() error {
	var errs []error
	errs = append(errs, o.ServerRunOptions.Validate()...)
	errs = append(errs, o.RecommendedOptions.Validate()...)
	return utilerrors.NewAggregate(errs)
}

// Complete 补全 ingate-apiserver 启动参数
func (o *Options) Complete() error {
	return o.ServerRunOptions.Complete()
}

// Config 创建 ingate-apiserver 配置
func (o *Options) Config() (*Config, error) {
	genericConfig := genericapiserver.NewRecommendedConfig(Codecs)
	if err := o.ServerRunOptions.ApplyTo(&genericConfig.Config); err != nil {
		return nil, err
	}
	if err := o.RecommendedOptions.ApplyTo(genericConfig); err != nil {
		return nil, err
	}

	return &Config{
		GenericConfig: genericConfig,
	}, nil
}

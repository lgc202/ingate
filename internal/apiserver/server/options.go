package server

import (
	"io"
	"net"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	netutils "k8s.io/utils/net"

	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	generatedopenapi "github.com/lgc202/ingate/pkg/generated/openapi"
)

const (
	// DefaultEtcdPathPrefix 表示 ingate-apiserver 在 etcd 中使用的默认前缀
	DefaultEtcdPathPrefix = "/registry/gateway.ingate.io"
	// DefaultCertDirectory 表示本地运行时默认生成自签名证书的目录
	DefaultCertDirectory = "_output/certificates"
)

// Options 表示 ingate-apiserver 启动参数
type Options struct {
	ServerRunOptions *genericoptions.ServerRunOptions
	SecureServing    *genericoptions.SecureServingOptionsWithLoopback
	Etcd             *genericoptions.EtcdOptions

	StdOut io.Writer
	StdErr io.Writer
}

// NewOptions 创建 ingate-apiserver 默认启动参数
func NewOptions(stdout, stderr io.Writer) *Options {
	secureServing := genericoptions.NewSecureServingOptions().WithLoopback()
	secureServing.BindAddress = netutils.ParseIPSloppy("127.0.0.1")
	secureServing.BindPort = 18443
	secureServing.ServerCert.CertDirectory = DefaultCertDirectory
	secureServing.ServerCert.PairName = "apiserver"

	storageCodec := Codecs.LegacyCodec(gatewayv1.SchemeGroupVersion)
	etcd := genericoptions.NewEtcdOptions(storagebackend.NewDefaultConfig(DefaultEtcdPathPrefix, storageCodec))
	etcd.StorageConfig.Transport.ServerList = []string{"http://127.0.0.1:2379"}
	etcd.DefaultStorageMediaType = runtime.ContentTypeJSON

	return &Options{
		ServerRunOptions: genericoptions.NewServerRunOptions(),
		SecureServing:    secureServing,
		Etcd:             etcd,
		StdOut:           stdout,
		StdErr:           stderr,
	}
}

// AddFlags 注册 ingate-apiserver 命令行参数
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	o.ServerRunOptions.AddUniversalFlags(flags)
	o.SecureServing.AddFlags(flags)
	o.Etcd.AddFlags(flags)
}

// Validate 校验 ingate-apiserver 启动参数
func (o *Options) Validate() error {
	var errs []error
	errs = append(errs, o.ServerRunOptions.Validate()...)
	errs = append(errs, o.SecureServing.Validate()...)
	errs = append(errs, o.Etcd.Validate()...)
	return utilerrors.NewAggregate(errs)
}

// Complete 补全 ingate-apiserver 启动参数
func (o *Options) Complete() error {
	if err := o.ServerRunOptions.Complete(); err != nil {
		return err
	}
	if err := o.ServerRunOptions.DefaultAdvertiseAddress(o.SecureServing.SecureServingOptions); err != nil {
		return err
	}
	advertiseAddress := o.ServerRunOptions.AdvertiseAddress
	if advertiseAddress == nil || advertiseAddress.IsUnspecified() {
		return nil
	}
	return o.SecureServing.MaybeDefaultWithSelfSignedCerts(
		advertiseAddress.String(),
		[]string{"localhost", "ingate.local"},
		[]net.IP{netutils.ParseIPSloppy("127.0.0.1")},
	)
}

// Config 创建 ingate-apiserver 配置
func (o *Options) Config() (*Config, error) {
	genericConfig := genericapiserver.NewRecommendedConfig(Codecs)
	if err := o.ServerRunOptions.ApplyTo(&genericConfig.Config); err != nil {
		return nil, err
	}
	if err := o.SecureServing.ApplyTo(&genericConfig.Config.SecureServing, &genericConfig.Config.LoopbackClientConfig); err != nil {
		return nil, err
	}
	if err := o.Etcd.ApplyTo(&genericConfig.Config); err != nil {
		return nil, err
	}

	openAPIDefinitions := generatedopenapi.GetOpenAPIDefinitions
	openAPINamer := openapinamer.NewDefinitionNamer(Scheme)
	genericConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(openAPIDefinitions, openAPINamer)
	genericConfig.OpenAPIConfig.Info.Title = "Ingate API Server"
	genericConfig.OpenAPIConfig.Info.Version = gatewayv1.SchemeGroupVersion.Version
	genericConfig.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(openAPIDefinitions, openAPINamer)
	genericConfig.OpenAPIV3Config.Info.Title = "Ingate API Server"
	genericConfig.OpenAPIV3Config.Info.Version = gatewayv1.SchemeGroupVersion.Version

	return &Config{GenericConfig: genericConfig}, nil
}

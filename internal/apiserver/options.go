package apiserver

import (
	"fmt"
	"net"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	apiopenapi "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/compatibility"

	gatewayapi "github.com/lgc202/ingate/pkg/apis/gateway"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	generatedopenapi "github.com/lgc202/ingate/pkg/generated/openapi"
)

const (
	defaultEtcdPathPrefix = "/registry/gateway.ingate.io"
	openAPITitle          = "Ingate API Server"
)

// Options 保存 API Server 的启动选项及其序列化依赖。
type Options struct {
	RecommendedOptions *genericoptions.RecommendedOptions

	scheme *runtime.Scheme
	codecs serializer.CodecFactory
}

// NewOptions 创建具有标准默认值的 API Server 启动选项。
func NewOptions() (*Options, error) {
	scheme, err := newScheme()
	if err != nil {
		return nil, fmt.Errorf("create API scheme: %w", err)
	}

	codecs := serializer.NewCodecFactory(scheme)
	recommendedOptions := genericoptions.NewRecommendedOptions(
		defaultEtcdPathPrefix,
		codecs.LegacyCodec(gatewayv1alpha1.SchemeGroupVersion),
	)

	// 请求使用外部版本；持久化统一写入当前存储版本。
	recommendedOptions.Etcd.StorageConfig.EncodeVersioner = runtime.NewMultiGroupVersioner(
		gatewayv1alpha1.SchemeGroupVersion,
		schema.GroupKind{Group: gatewayapi.GroupName},
	)

	return &Options{
		RecommendedOptions: recommendedOptions,
		scheme:             scheme,
		codecs:             codecs,
	}, nil
}

// AddFlags 注册 Generic API Server 的标准启动参数。
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	o.RecommendedOptions.AddFlags(fs)
}

// Validate 校验 API Server 的启动选项。
func (o *Options) Validate() error {
	return utilerrors.NewAggregate(o.RecommendedOptions.Validate())
}

// Config 将已校验的选项应用到尚可修改的 Server 配置。
func (o *Options) Config() (*Config, error) {
	// 未配置证书时由上游生成 localhost/127.0.0.1 自签名证书；监听地址仍由启动选项决定。
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts(
		"localhost",
		nil,
		[]net.IP{net.ParseIP("127.0.0.1")},
	); err != nil {
		return nil, fmt.Errorf("default serving certificate: %w", err)
	}

	genericConfig := genericapiserver.NewRecommendedConfig(o.codecs)
	genericConfig.EffectiveVersion = compatibility.DefaultBuildEffectiveVersion()
	genericConfig.FeatureGate = o.RecommendedOptions.FeatureGate

	definitionNamer := apiopenapi.NewDefinitionNamer(o.scheme)
	genericConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(
		generatedopenapi.GetOpenAPIDefinitions,
		definitionNamer,
	)
	genericConfig.OpenAPIConfig.Info.Title = openAPITitle
	genericConfig.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(
		generatedopenapi.GetOpenAPIDefinitions,
		definitionNamer,
	)
	genericConfig.OpenAPIV3Config.Info.Title = openAPITitle

	// ApplyTo 准备存储、认证和 HTTPS Listener，但不启动请求处理循环。
	if err := o.RecommendedOptions.ApplyTo(genericConfig); err != nil {
		return nil, fmt.Errorf("apply recommended API server options: %w", err)
	}

	return &Config{GenericConfig: genericConfig, Scheme: o.scheme}, nil
}

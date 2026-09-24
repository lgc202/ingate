package controllermanager

import (
	"fmt"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

const (
	componentName                 = "ingate-controller-manager"
	defaultXDSBindAddress         = ":18000"
	defaultMetricsBindAddress     = ":8080"
	defaultHealthProbeBindAddress = ":8081"
)

// Options 保存 Controller Manager 的启动参数。
type Options struct {
	MasterURL              string
	Kubeconfig             string
	XDSBindAddress         string
	MetricsBindAddress     string
	HealthProbeBindAddress string
}

// NewOptions 创建采用标准监听地址的启动选项。
func NewOptions() *Options {
	return &Options{
		XDSBindAddress:         defaultXDSBindAddress,
		MetricsBindAddress:     defaultMetricsBindAddress,
		HealthProbeBindAddress: defaultHealthProbeBindAddress,
	}
}

// AddFlags 注册 API 连接、xDS 和探针相关的启动参数。
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.MasterURL, "master", o.MasterURL, "API server address; overrides kubeconfig.")
	fs.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "Path to the API server kubeconfig file.")
	fs.StringVar(&o.XDSBindAddress, "xds-bind-address", o.XDSBindAddress, "Address for the xDS ADS server.")
	fs.StringVar(
		&o.MetricsBindAddress, "metrics-bind-address", o.MetricsBindAddress,
		"Address for Prometheus metrics; use 0 to disable.",
	)
	fs.StringVar(
		&o.HealthProbeBindAddress, "health-probe-bind-address", o.HealthProbeBindAddress,
		"Address for health and readiness probes; use 0 to disable.",
	)
}

// Config 根据启动选项创建 Manager 配置。
func (o *Options) Config() (*Config, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags(o.MasterURL, o.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("build API server client config: %w", err)
	}
	restConfig = rest.AddUserAgent(restConfig, componentName)

	scheme := runtime.NewScheme()
	if err := gatewayv1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("register Gateway API types: %w", err)
	}

	return &Config{
		RESTConfig:     restConfig,
		XDSBindAddress: o.XDSBindAddress,
		ManagerOptions: ctrl.Options{
			Scheme:                 scheme,
			Metrics:                metricsserver.Options{BindAddress: o.MetricsBindAddress},
			HealthProbeBindAddress: o.HealthProbeBindAddress,
		},
	}, nil
}

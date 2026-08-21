package server

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	netutils "k8s.io/utils/net"

	"github.com/lgc202/ingate/internal/apiserver/conf"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	generatedopenapi "github.com/lgc202/ingate/pkg/generated/openapi"
)

const serverName = "ingate-apiserver"

// Server 让 Kubernetes Generic API Server 接入 Kratos 生命周期
type Server struct {
	generic  *genericapiserver.GenericAPIServer
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

// New 根据进程配置创建 Generic API Server
func New(httpConfig *conf.Server_HTTP, etcdConfig *conf.Data_Etcd) (*Server, error) {
	serverRunOptions := genericoptions.NewServerRunOptions()
	secureServing, err := newSecureServingOptions(httpConfig)
	if err != nil {
		return nil, err
	}
	etcd := newEtcdOptions(etcdConfig)
	if err := serverRunOptions.Complete(); err != nil {
		return nil, fmt.Errorf("complete API server options: %w", err)
	}
	if err := serverRunOptions.DefaultAdvertiseAddress(secureServing.SecureServingOptions); err != nil {
		return nil, fmt.Errorf("set API server advertise address: %w", err)
	}
	advertiseAddress := serverRunOptions.AdvertiseAddress
	if advertiseAddress != nil && !advertiseAddress.IsUnspecified() {
		if err := secureServing.MaybeDefaultWithSelfSignedCerts(
			advertiseAddress.String(),
			[]string{"localhost", "ingate.local"},
			[]net.IP{netutils.ParseIPSloppy("127.0.0.1")},
		); err != nil {
			return nil, fmt.Errorf("create API server serving certificate: %w", err)
		}
	}

	var optionErrors []error
	optionErrors = append(optionErrors, serverRunOptions.Validate()...)
	optionErrors = append(optionErrors, secureServing.Validate()...)
	optionErrors = append(optionErrors, etcd.Validate()...)
	if err := utilerrors.NewAggregate(optionErrors); err != nil {
		return nil, fmt.Errorf("validate API server options: %w", err)
	}

	genericConfig := genericapiserver.NewRecommendedConfig(Codecs)
	if err := serverRunOptions.ApplyTo(&genericConfig.Config); err != nil {
		return nil, fmt.Errorf("apply API server options: %w", err)
	}
	if err := secureServing.ApplyTo(&genericConfig.Config.SecureServing, &genericConfig.Config.LoopbackClientConfig); err != nil {
		return nil, fmt.Errorf("apply API server secure serving options: %w", err)
	}
	if err := etcd.ApplyTo(&genericConfig.Config); err != nil {
		return nil, fmt.Errorf("apply API server etcd options: %w", err)
	}
	configureOpenAPI(genericConfig)

	completedConfig := genericConfig.Complete()
	genericServer, err := completedConfig.New(serverName, genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, fmt.Errorf("create Generic API Server: %w", err)
	}

	if err := installResources(genericServer, completedConfig); err != nil {
		return nil, fmt.Errorf("install API resources: %w", err)
	}

	return &Server{
		generic: genericServer,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}, nil
}

func newSecureServingOptions(config *conf.Server_HTTP) (*genericoptions.SecureServingOptionsWithLoopback, error) {
	host, portText, err := net.SplitHostPort(config.GetAddr())
	if err != nil {
		return nil, fmt.Errorf("parse API server address %q: %w", config.GetAddr(), err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return nil, fmt.Errorf("parse API server port %q: %w", portText, err)
	}

	options := genericoptions.NewSecureServingOptions().WithLoopback()
	options.BindAddress = netutils.ParseIPSloppy(host)
	options.BindPort = port
	options.ServerCert.CertDirectory = config.GetCertDirectory()
	options.ServerCert.PairName = "apiserver"
	return options, nil
}

func newEtcdOptions(config *conf.Data_Etcd) *genericoptions.EtcdOptions {
	storageCodec := Codecs.LegacyCodec(gatewayv1.SchemeGroupVersion)
	options := genericoptions.NewEtcdOptions(storagebackend.NewDefaultConfig(config.GetPrefix(), storageCodec))
	options.StorageConfig.Transport.ServerList = append([]string(nil), config.GetEndpoints()...)
	options.DefaultStorageMediaType = runtime.ContentTypeJSON
	return options
}

func configureOpenAPI(config *genericapiserver.RecommendedConfig) {
	openAPIDefinitions := generatedopenapi.GetOpenAPIDefinitions
	openAPINamer := openapinamer.NewDefinitionNamer(Scheme)
	config.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(openAPIDefinitions, openAPINamer)
	config.OpenAPIConfig.Info.Title = "Ingate API Server"
	config.OpenAPIConfig.Info.Version = gatewayv1.SchemeGroupVersion.Version
	config.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(openAPIDefinitions, openAPINamer)
	config.OpenAPIV3Config.Info.Title = "Ingate API Server"
	config.OpenAPIV3Config.Info.Version = gatewayv1.SchemeGroupVersion.Version
}

// Start 阻塞运行 Generic API Server，直到 Kratos 调用 Stop
func (s *Server) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer close(s.done)
	go func() {
		select {
		case <-s.stop:
			cancel()
		case <-runCtx.Done():
		}
	}()
	return s.generic.PrepareRun().RunWithContext(runCtx)
}

// Stop 通知 Generic API Server 停止并等待在途请求完成
func (s *Server) Stop(ctx context.Context) error {
	s.stopOnce.Do(func() { close(s.stop) })
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

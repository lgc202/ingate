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
	host, portText, err := net.SplitHostPort(httpConfig.GetAddr())
	if err != nil {
		return nil, fmt.Errorf("parse API server address %q: %w", httpConfig.GetAddr(), err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return nil, fmt.Errorf("parse API server port %q: %w", portText, err)
	}

	serverRunOptions := genericoptions.NewServerRunOptions()
	secureServing := genericoptions.NewSecureServingOptions().WithLoopback()
	secureServing.BindAddress = netutils.ParseIPSloppy(host)
	secureServing.BindPort = port
	secureServing.ServerCert.CertDirectory = httpConfig.GetCertDirectory()
	secureServing.ServerCert.PairName = "apiserver"

	storageCodec := Codecs.LegacyCodec(gatewayv1.SchemeGroupVersion)
	etcd := genericoptions.NewEtcdOptions(storagebackend.NewDefaultConfig(etcdConfig.GetPrefix(), storageCodec))
	etcd.StorageConfig.Transport.ServerList = append([]string(nil), etcdConfig.GetEndpoints()...)
	etcd.DefaultStorageMediaType = runtime.ContentTypeJSON

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

	openAPIDefinitions := generatedopenapi.GetOpenAPIDefinitions
	openAPINamer := openapinamer.NewDefinitionNamer(Scheme)
	genericConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(openAPIDefinitions, openAPINamer)
	genericConfig.OpenAPIConfig.Info.Title = "Ingate API Server"
	genericConfig.OpenAPIConfig.Info.Version = gatewayv1.SchemeGroupVersion.Version
	genericConfig.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(openAPIDefinitions, openAPINamer)
	genericConfig.OpenAPIV3Config.Info.Title = "Ingate API Server"
	genericConfig.OpenAPIV3Config.Info.Version = gatewayv1.SchemeGroupVersion.Version

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

// Start 阻塞运行 Generic API Server，直到 Kratos 调用 Stop
func (s *Server) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-s.stop:
			cancel()
		case <-runCtx.Done():
		}
	}()
	runErr := s.generic.PrepareRun().RunWithContext(runCtx)
	close(s.done)
	return runErr
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

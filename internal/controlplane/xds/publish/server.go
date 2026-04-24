package publish

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	envoydiscoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiutil "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	xdsads "github.com/lgc202/ingate/internal/controlplane/xds/ads"
	xdscache "github.com/lgc202/ingate/internal/controlplane/xds/cache"
	"github.com/lgc202/ingate/internal/controlplane/xds/translate"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	configsyncv1 "github.com/lgc202/ingate/pkg/generated/proto/ingate/configsync/v1"
	discoveryv1 "github.com/lgc202/ingate/pkg/generated/proto/ingate/discovery/v1"
)

const (
	ConditionProgrammed = "Programmed"
	reasonProgrammed    = "Programmed"
	reasonPublishFailed = "PublishFailed"
	shutdownTimeout     = 10 * time.Second
)

type Server struct {
	bindAddress string
	cache       *xdscache.Cache
	client      clientset.Interface
	now         func() metav1.Time
	grpcServer  *grpc.Server
}

func NewServer(bindAddress string, cache *xdscache.Cache, client clientset.Interface) *Server {
	grpcServer := grpc.NewServer()
	discoveryv1.RegisterDiscoveryServiceServer(grpcServer, &discoveryService{cache: cache})
	configsyncv1.RegisterConfigSyncServiceServer(grpcServer, &configSyncService{cache: cache})
	envoydiscoveryv3.RegisterAggregatedDiscoveryServiceServer(grpcServer, xdsads.NewService(cache))
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	reflection.Register(grpcServer)

	return &Server{
		bindAddress: bindAddress,
		cache:       cache,
		client:      client,
		now:         func() metav1.Time { return metav1.NewTime(time.Now()) },
		grpcServer:  grpcServer,
	}
}

func (s *Server) Run(ctx context.Context) error {
	if s == nil || s.grpcServer == nil {
		return fmt.Errorf("publish server is not initialized")
	}
	if s.bindAddress == "" {
		return fmt.Errorf("publish server bind address must not be empty")
	}

	listener, err := net.Listen("tcp", s.bindAddress)
	if err != nil {
		return err
	}
	defer listener.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.grpcServer.Serve(listener)
	}()

	select {
	case err := <-errCh:
		if err == nil || errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return err
	case <-ctx.Done():
		stopped := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(shutdownTimeout):
			s.grpcServer.Stop()
		}
		return nil
	}
}

func (s *Server) Publish(ctx context.Context, gateway *gatewayv1alpha1.Gateway, runtimeConfig *translate.RuntimeConfig) error {
	if s == nil || s.cache == nil {
		return fmt.Errorf("publish server is not initialized")
	}
	if gateway == nil {
		return fmt.Errorf("gateway must not be nil")
	}
	if runtimeConfig == nil {
		return fmt.Errorf("runtime config must not be nil")
	}

	s.cache.Upsert(xdscache.Snapshot{
		Key:             shared.NewObjectKey(gateway.Namespace, gateway.Name),
		SourceVersion:   gateway.ResourceVersion,
		PublishVersion:  runtimeConfig.Version,
		Runtime:         runtimeConfig,
		EffectiveConfig: effectiveConfigFromRuntime(runtimeConfig),
	})

	if s.client == nil {
		return nil
	}

	return s.updateProgrammedCondition(
		ctx,
		gateway,
		metav1.ConditionTrue,
		reasonProgrammed,
		fmt.Sprintf("published gateway version %s", runtimeConfig.Version),
	)
}

func (s *Server) Delete(_ context.Context, key shared.ObjectKey) error {
	if s == nil || s.cache == nil {
		return fmt.Errorf("publish server is not initialized")
	}
	s.cache.Delete(key)
	return nil
}

func (s *Server) PublishFailure(ctx context.Context, gateway *gatewayv1alpha1.Gateway, publishErr error) error {
	if s == nil {
		return fmt.Errorf("publish server is not initialized")
	}
	if gateway == nil {
		return nil
	}
	if s.client == nil {
		return publishErr
	}

	message := "failed to publish gateway"
	if publishErr != nil {
		message = publishErr.Error()
	}
	return s.updateProgrammedCondition(ctx, gateway, metav1.ConditionFalse, reasonPublishFailed, message)
}

func (s *Server) updateProgrammedCondition(ctx context.Context, gateway *gatewayv1alpha1.Gateway, status metav1.ConditionStatus, reason, message string) error {
	existing := apiutil.FindStatusCondition(gateway.Status.Conditions, ConditionProgrammed)
	if existing != nil &&
		existing.Status == status &&
		existing.Reason == reason &&
		existing.Message == message &&
		gateway.Status.ObservedGeneration == gateway.Generation {
		return nil
	}

	updated := gateway.DeepCopy()
	updated.Status.ObservedGeneration = updated.Generation
	apiutil.SetStatusCondition(&updated.Status.Conditions, metav1.Condition{
		Type:               ConditionProgrammed,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: s.now(),
	})

	_, err := s.client.GatewayV1alpha1().Gateways().UpdateStatus(ctx, updated, metav1.UpdateOptions{})
	if apierrors.IsNotFound(err) {
		_, err = s.client.GatewayV1alpha1().Gateways().Update(ctx, updated, metav1.UpdateOptions{})
	}
	return err
}

func effectiveConfigFromRuntime(runtimeConfig *translate.RuntimeConfig) *configsyncv1.EffectiveConfig {
	if runtimeConfig == nil {
		return nil
	}

	config := &configsyncv1.EffectiveConfig{
		Version:   runtimeConfig.Version,
		Listeners: make([]*configsyncv1.Listener, 0, len(runtimeConfig.Listeners)),
		Routes:    make([]*configsyncv1.Route, 0, len(runtimeConfig.Routes)),
		Backends:  make([]*configsyncv1.Backend, 0, len(runtimeConfig.Backends)),
	}

	for _, listener := range runtimeConfig.Listeners {
		config.Listeners = append(config.Listeners, &configsyncv1.Listener{
			Name:      listener.Name,
			Protocol:  listener.Protocol,
			Port:      listener.Port,
			Hostnames: append([]string(nil), listener.Hostnames...),
		})
	}

	for _, route := range runtimeConfig.Routes {
		item := &configsyncv1.Route{
			Name:         route.Name,
			Hostnames:    append([]string(nil), route.Hostnames...),
			PathPrefixes: collectPathPrefixes(route.Rules),
			Backends:     collectBackendRefs(route.Rules),
		}
		config.Routes = append(config.Routes, item)
	}

	for _, backend := range runtimeConfig.Backends {
		item := &configsyncv1.Backend{
			Name:        backend.Name,
			Type:        backend.Type,
			DefaultPort: backend.DefaultPort,
			Endpoints:   make([]*configsyncv1.Endpoint, 0, len(backend.Endpoints)),
		}
		for _, endpoint := range backend.Endpoints {
			item.Endpoints = append(item.Endpoints, &configsyncv1.Endpoint{
				Address: endpoint.Address,
				Port:    endpoint.Port,
				Weight:  endpoint.Weight,
				Healthy: endpoint.Healthy,
			})
		}
		config.Backends = append(config.Backends, item)
	}

	return config
}

func collectPathPrefixes(rules []translate.RuntimeRouteRule) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, rule := range rules {
		for _, match := range rule.Matches {
			if match.Path == nil || match.Path.Value == "" {
				continue
			}
			if _, ok := seen[match.Path.Value]; ok {
				continue
			}
			seen[match.Path.Value] = struct{}{}
			out = append(out, match.Path.Value)
		}
	}
	return out
}

func collectBackendRefs(rules []translate.RuntimeRouteRule) []*configsyncv1.BackendRef {
	out := make([]*configsyncv1.BackendRef, 0)
	for _, rule := range rules {
		for _, backendRef := range rule.BackendRefs {
			out = append(out, &configsyncv1.BackendRef{
				Name:   backendRef.Name,
				Port:   backendRef.Port,
				Weight: backendRef.Weight,
			})
		}
	}
	return out
}

type discoveryService struct {
	discoveryv1.UnimplementedDiscoveryServiceServer
	cache *xdscache.Cache
}

type configSyncService struct {
	configsyncv1.UnimplementedConfigSyncServiceServer
	cache *xdscache.Cache
}

func (s *discoveryService) Resolve(_ context.Context, req *discoveryv1.ResolveRequest) (*discoveryv1.ResolveResponse, error) {
	if req == nil || req.GetBackendName() == "" {
		return &discoveryv1.ResolveResponse{}, nil
	}
	if s == nil || s.cache == nil {
		return &discoveryv1.ResolveResponse{}, nil
	}

	backend, ok := s.cache.ResolveBackend(req.GetBackendName(), req.GetBackendType())
	if !ok {
		return &discoveryv1.ResolveResponse{}, nil
	}

	response := &discoveryv1.ResolveResponse{Endpoints: make([]*discoveryv1.Endpoint, 0, len(backend.Endpoints))}
	for _, endpoint := range backend.Endpoints {
		response.Endpoints = append(response.Endpoints, &discoveryv1.Endpoint{
			Address: endpoint.Address,
			Port:    endpoint.Port,
			Weight:  endpoint.Weight,
			Healthy: endpoint.Healthy,
		})
	}
	return response, nil
}

func (s *configSyncService) GetConfig(_ context.Context, req *configsyncv1.GetConfigRequest) (*configsyncv1.GetConfigResponse, error) {
	if req == nil || req.GetGatewayKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "gateway_key must not be empty")
	}
	if s == nil || s.cache == nil {
		return nil, status.Error(codes.Unavailable, "config cache is not initialized")
	}

	key, err := shared.ParseObjectKey(req.GetGatewayKey())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "parse gateway_key: %v", err)
	}

	snapshot, ok := s.cache.Get(key)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "gateway %q is not published", key.String())
	}

	resp := &configsyncv1.GetConfigResponse{
		GatewayKey:     key.String(),
		SourceVersion:  snapshot.SourceVersion,
		PublishVersion: snapshot.PublishVersion,
		Config:         snapshot.EffectiveConfig,
	}
	if !snapshot.UpdatedAt.IsZero() {
		resp.UpdatedAt = snapshot.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return resp, nil
}

func (s *configSyncService) ListConfigs(_ context.Context, _ *configsyncv1.ListConfigsRequest) (*configsyncv1.ListConfigsResponse, error) {
	if s == nil || s.cache == nil {
		return nil, status.Error(codes.Unavailable, "config cache is not initialized")
	}

	snapshots := s.cache.List()
	resp := &configsyncv1.ListConfigsResponse{
		Items: make([]*configsyncv1.PublishedConfig, 0, len(snapshots)),
	}
	for _, snapshot := range snapshots {
		item := &configsyncv1.PublishedConfig{
			GatewayKey:     snapshot.Key.String(),
			SourceVersion:  snapshot.SourceVersion,
			PublishVersion: snapshot.PublishVersion,
		}
		if !snapshot.UpdatedAt.IsZero() {
			item.UpdatedAt = snapshot.UpdatedAt.UTC().Format(time.RFC3339)
		}
		resp.Items = append(resp.Items, item)
	}
	return resp, nil
}

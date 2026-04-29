package server

import (
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// adsServer 是 Envoy ADS 协议入口，后续会从 snapshotStore 读取配置响应 Envoy
type adsServer struct {
	discoveryv3.UnimplementedAggregatedDiscoveryServiceServer
	store *snapshotStore
}

func newADSServer(store *snapshotStore) discoveryv3.AggregatedDiscoveryServiceServer {
	return &adsServer{store: store}
}

func (s *adsServer) StreamAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	return status.Error(codes.Unimplemented, "ADS stream is not implemented yet")
}

func (s *adsServer) DeltaAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	return status.Error(codes.Unimplemented, "ADS delta stream is not implemented yet")
}

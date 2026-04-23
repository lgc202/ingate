package ads

import (
	"context"
	"fmt"
	"io"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	xdscache "github.com/lgc202/ingate/internal/controlplane/xds/cache"
)

type Service struct {
	discoveryv3.UnimplementedAggregatedDiscoveryServiceServer

	cache *xdscache.Cache
}

func NewService(cache *xdscache.Cache) *Service {
	return &Service{cache: cache}
}

func (s *Service) StreamAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		resp, err := s.buildResponse(req)
		if err != nil {
			return err
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func (s *Service) DeltaAggregatedResources(discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	return status.Error(codes.Unimplemented, "delta ADS is not implemented")
}

func (s *Service) FetchAggregatedResources(_ context.Context, req *discoveryv3.DiscoveryRequest) (*discoveryv3.DiscoveryResponse, error) {
	return s.buildResponse(req)
}

func (s *Service) buildResponse(req *discoveryv3.DiscoveryRequest) (*discoveryv3.DiscoveryResponse, error) {
	if s == nil || s.cache == nil {
		return nil, status.Error(codes.Unavailable, "ads cache is not initialized")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "discovery request must not be nil")
	}

	typeURL, err := TypeURLForAlias(req.GetTypeUrl())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	snapshot, err := s.selectSnapshot(req.GetNode())
	if err != nil {
		return nil, err
	}

	resources, err := BuildResources(snapshot, typeURL, req.GetResourceNames())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build %s resources: %v", typeURL, err)
	}

	version := snapshot.PublishVersion
	if strings.TrimSpace(version) == "" {
		version = snapshot.SourceVersion
	}
	if strings.TrimSpace(version) == "" {
		version = "0"
	}

	return &discoveryv3.DiscoveryResponse{
		VersionInfo: version,
		TypeUrl:     typeURL,
		Nonce:       fmt.Sprintf("%s/%s", version, shortType(typeURL)),
		Resources:   resources,
	}, nil
}

func (s *Service) selectSnapshot(node *corev3.Node) (xdscache.Snapshot, error) {
	nodeID := ""
	if node != nil {
		nodeID = strings.TrimSpace(node.GetId())
	}
	if nodeID != "" {
		key, err := shared.ParseObjectKey(nodeID)
		if err != nil {
			return xdscache.Snapshot{}, status.Errorf(codes.InvalidArgument, "parse node.id: %v", err)
		}
		snapshot, ok := s.cache.Get(key)
		if !ok {
			return xdscache.Snapshot{}, status.Errorf(codes.NotFound, "published gateway %q not found", key.String())
		}
		return snapshot, nil
	}

	snapshots := s.cache.List()
	switch len(snapshots) {
	case 0:
		return xdscache.Snapshot{}, status.Error(codes.NotFound, "no published gateways available")
	case 1:
		return snapshots[0], nil
	default:
		return xdscache.Snapshot{}, status.Error(codes.InvalidArgument, "node.id must identify the published gateway when multiple snapshots exist")
	}
}

func shortType(typeURL string) string {
	switch typeURL {
	case "type.googleapis.com/envoy.config.listener.v3.Listener":
		return "lds"
	case "type.googleapis.com/envoy.config.route.v3.RouteConfiguration":
		return "rds"
	case "type.googleapis.com/envoy.config.cluster.v3.Cluster":
		return "cds"
	case "type.googleapis.com/envoy.config.endpoint.v3.ClusterLoadAssignment":
		return "eds"
	default:
		return "xds"
	}
}

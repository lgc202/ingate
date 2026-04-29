package server

import (
	"fmt"
	"io"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// adsServer 是 Envoy ADS 协议入口，后续会从 snapshotStore 读取配置响应 Envoy
type adsServer struct {
	discoveryv3.UnimplementedAggregatedDiscoveryServiceServer
	store  *snapshotStore
	stdout io.Writer
}

func newADSServer(store *snapshotStore, stdout io.Writer) discoveryv3.AggregatedDiscoveryServiceServer {
	return &adsServer{store: store, stdout: stdout}
}

// StreamAggregatedResources 处理 Envoy ADS 的 State-of-the-World 流
// Envoy 会在同一个双向流里按 type_url 订阅 LDS/CDS/RDS/EDS 等资源，并用后续请求 ACK/NACK 上一次响应
// 当前阶段只接收并记录请求，不下发资源
func (s *adsServer) StreamAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	for {
		request, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		s.logRequest("stream", request)
	}
}

// DeltaAggregatedResources 处理 Envoy ADS 的增量流
// 增量 xDS 会显式携带资源订阅变化和已知版本，后续再实现；当前先明确返回未实现
func (s *adsServer) DeltaAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	return status.Error(codes.Unimplemented, "ADS delta stream is not implemented yet")
}

func (s *adsServer) logRequest(streamType string, request *discoveryv3.DiscoveryRequest) {
	nodeID := ""
	if request.GetNode() != nil {
		nodeID = request.GetNode().GetId()
	}

	fmt.Fprintf(s.stdout, "ads request stream=%s node=%s type=%s version=%s nonce=%s resources=%d snapshots=%d\n",
		streamType,
		nodeID,
		request.GetTypeUrl(),
		request.GetVersionInfo(),
		request.GetResponseNonce(),
		len(request.GetResourceNames()),
		s.store.Count(),
	)
}

func registerADSServer(grpcServer *grpc.Server, store *snapshotStore, stdout io.Writer) {
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(grpcServer, newADSServer(store, stdout))
}

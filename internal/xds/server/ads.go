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
	responses responseBuilder
	store     *snapshotStore
	stdout    io.Writer
}

func newADSServer(store *snapshotStore, stdout io.Writer) discoveryv3.AggregatedDiscoveryServiceServer {
	return &adsServer{responses: newResponseBuilder(store), store: store, stdout: stdout}
}

// StreamAggregatedResources 处理 Envoy ADS 的 State-of-the-World 流
// Envoy 会在同一个双向流里按 type_url 订阅 LDS/CDS/RDS/EDS 等资源，并用后续请求 ACK/NACK 上一次响应
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
		s.logAcknowledgement(request)
		response, ok, err := s.responses.Build(request)
		if err != nil {
			fmt.Fprintf(s.stdout, "ads response skipped type=%s error=%v\n", request.GetTypeUrl(), err)
			continue
		}
		if !ok {
			continue
		}
		if err := stream.Send(response); err != nil {
			return err
		}
		fmt.Fprintf(s.stdout, "ads response sent type=%s version=%s nonce=%s resources=%d\n", response.GetTypeUrl(), response.GetVersionInfo(), response.GetNonce(), len(response.GetResources()))
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

func (s *adsServer) logAcknowledgement(request *discoveryv3.DiscoveryRequest) {
	if request.GetResponseNonce() == "" {
		return
	}

	errorDetail := request.GetErrorDetail()
	if errorDetail == nil {
		fmt.Fprintf(s.stdout, "ads ack type=%s version=%s nonce=%s\n",
			request.GetTypeUrl(),
			request.GetVersionInfo(),
			request.GetResponseNonce(),
		)
		return
	}

	fmt.Fprintf(s.stdout, "ads nack type=%s version=%s nonce=%s code=%d message=%q\n",
		request.GetTypeUrl(),
		request.GetVersionInfo(),
		request.GetResponseNonce(),
		errorDetail.GetCode(),
		errorDetail.GetMessage(),
	)
}

func registerADSServer(grpcServer *grpc.Server, store *snapshotStore, stdout io.Writer) {
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(grpcServer, newADSServer(store, stdout))
}

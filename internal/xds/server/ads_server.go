package server

import (
	"io"
	"log/slog"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// adsServer 是 Envoy ADS 协议入口，从 snapshotStore 读取配置并通过 ADS 流响应 Envoy
type adsServer struct {
	discoveryv3.UnimplementedAggregatedDiscoveryServiceServer
	responses responseBuilder
	store     *snapshotStore
	updates   *adsUpdateNotifier
	logger    *slog.Logger
}

func newADSServer(store *snapshotStore, logger *slog.Logger) *adsServer {
	return &adsServer{responses: newResponseBuilder(store), store: store, updates: newADSUpdateNotifier(), logger: logger}
}

// StreamAggregatedResources 处理 Envoy ADS 的 State-of-the-World 流
// Envoy 会在同一个双向流里按 type_url 订阅 LDS/CDS/RDS/EDS 等资源，并用后续请求 ACK/NACK 上一次响应
func (s *adsServer) StreamAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	updates, unsubscribe := s.updates.Subscribe()
	defer unsubscribe()

	requests := make(chan adsStreamRequest, 1)
	go s.receiveStreamRequests(stream, requests)

	state := newADSStreamState()
	for {
		select {
		case received := <-requests:
			if received.err != nil {
				if received.err == io.EOF {
					return nil
				}
				return received.err
			}
			if err := s.handleStreamRequest(stream, &state, received.request); err != nil {
				return err
			}
		case <-updates:
			if err := s.pushSubscribedResponses(stream, &state); err != nil {
				return err
			}
		}
	}
}

// NotifySnapshotsChanged 唤醒已连接的 ADS stream，让它们按当前订阅类型主动推送新 snapshot
func (s *adsServer) NotifySnapshotsChanged() {
	s.updates.Notify()
}

// DeltaAggregatedResources 处理 Envoy ADS 的增量流
// 增量 xDS 会显式携带资源订阅变化和已知版本，后续再实现；当前先明确返回未实现
func (s *adsServer) DeltaAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	return status.Error(codes.Unimplemented, "ADS delta stream is not implemented yet")
}

func (s *adsServer) receiveStreamRequests(stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer, requests chan<- adsStreamRequest) {
	for {
		request, err := stream.Recv()
		requests <- adsStreamRequest{request: request, err: err}
		if err != nil {
			return
		}
	}
}

func (s *adsServer) handleStreamRequest(stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer, state *adsStreamState, request *discoveryv3.DiscoveryRequest) error {
	s.logRequest("stream", request)
	s.logAcknowledgement(request)
	state.recordRequest(request)

	response, ok, err := s.responses.Build(request)
	if err != nil {
		s.logger.Warn("ads response skipped",
			"type_url", request.GetTypeUrl(),
			"err", err,
		)
		return nil
	}
	if !ok {
		return nil
	}
	if state.isAcknowledged(request, response) {
		s.logger.Debug("ads response unchanged",
			"type_url", response.GetTypeUrl(),
			"version", response.GetVersionInfo(),
			"nonce", response.GetNonce(),
		)
		return nil
	}
	return s.sendResponse(stream, state, response)
}

func (s *adsServer) pushSubscribedResponses(stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer, state *adsStreamState) error {
	for _, typeURL := range state.subscribedTypes() {
		request := state.requests[typeURL]
		response, ok, err := s.responses.Build(request)
		if err != nil {
			s.logger.Warn("ads push skipped",
				"type_url", request.GetTypeUrl(),
				"err", err,
			)
			continue
		}
		if !ok || state.hasSent(response) {
			continue
		}
		if err := s.sendResponse(stream, state, response); err != nil {
			return err
		}
	}
	return nil
}

func (s *adsServer) sendResponse(stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer, state *adsStreamState, response *discoveryv3.DiscoveryResponse) error {
	if err := stream.Send(response); err != nil {
		return err
	}
	state.record(response)
	s.logger.Info("ads response sent",
		"type_url", response.GetTypeUrl(),
		"version", response.GetVersionInfo(),
		"nonce", response.GetNonce(),
		"resources", len(response.GetResources()),
	)
	return nil
}

func (s *adsServer) logRequest(streamType string, request *discoveryv3.DiscoveryRequest) {
	nodeID := ""
	if request.GetNode() != nil {
		nodeID = request.GetNode().GetId()
	}

	s.logger.Debug("ads request received",
		"stream", streamType,
		"node_id", nodeID,
		"type_url", request.GetTypeUrl(),
		"version", request.GetVersionInfo(),
		"nonce", request.GetResponseNonce(),
		"resources", len(request.GetResourceNames()),
		"snapshots", s.store.Count(),
	)
}

func (s *adsServer) logAcknowledgement(request *discoveryv3.DiscoveryRequest) {
	if request.GetResponseNonce() == "" {
		return
	}

	errorDetail := request.GetErrorDetail()
	if errorDetail == nil {
		s.logger.Debug("ads ack received",
			"type_url", request.GetTypeUrl(),
			"version", request.GetVersionInfo(),
			"nonce", request.GetResponseNonce(),
		)
		return
	}

	s.logger.Warn("ads nack received",
		"type_url", request.GetTypeUrl(),
		"version", request.GetVersionInfo(),
		"nonce", request.GetResponseNonce(),
		"code", errorDetail.GetCode(),
		"message", errorDetail.GetMessage(),
	)
}

func registerADSServer(grpcServer *grpc.Server, store *snapshotStore, logger *slog.Logger) *adsServer {
	server := newADSServer(store, logger)
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(grpcServer, server)
	return server
}

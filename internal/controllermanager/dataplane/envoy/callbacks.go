package envoy

import (
	"context"
	"sync"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/go-logr/logr"
)

// callbacks 关联 SotW ADS 的 ACK/NACK；不参与配置发布或就绪判定。
type callbacks struct {
	logger logr.Logger

	// mu 保护 streams 及其中的状态；日志在解锁后写入。
	mu      sync.Mutex
	streams map[int64]*streamState
}

type streamState struct {
	nodeID    string
	responses map[string]responseState
}

type responseState struct {
	nonce   string
	version string
}

func newCallbacks(logger logr.Logger) *callbacks {
	return &callbacks{
		logger:  logger,
		streams: make(map[int64]*streamState),
	}
}

// OnStreamOpen 为新流分配独立的响应关联状态。
func (c *callbacks) OnStreamOpen(_ context.Context, streamID int64, _ string) error {
	c.mu.Lock()
	c.streams[streamID] = &streamState{responses: make(map[string]responseState)}
	c.mu.Unlock()

	return nil
}

// OnStreamClosed 删除流状态，重连不继承旧流的 nonce。
func (c *callbacks) OnStreamClosed(streamID int64, _ *corev3.Node) {
	c.mu.Lock()
	delete(c.streams, streamID)
	c.mu.Unlock()
}

// OnStreamRequest 只确认同流、同类型、同 nonce 的响应；NACK 不终止 ADS 流。
func (c *callbacks) OnStreamRequest(streamID int64, request *discoveryv3.DiscoveryRequest) error {
	nodeID, response := c.responseForRequest(streamID, request)
	if request.GetResponseNonce() == "" || !supportedResourceType(request.GetTypeUrl()) {
		return nil
	}

	logger := c.logger.WithValues(
		"stream_id", streamID,
		"node_id", nodeID,
		"type_url", request.GetTypeUrl(),
		"nonce", request.GetResponseNonce(),
		"client_version", request.GetVersionInfo(),
	)
	if response.nonce != request.GetResponseNonce() {
		logger.V(1).Info("xDS feedback has no matching response", "result", "unmatched")
		return nil
	}

	if detail := request.GetErrorDetail(); detail != nil {
		// ErrorDetail.Message 可能包含配置正文或凭据，只记录错误码。
		logger.Error(nil, "xDS configuration rejected",
			"result", "nack",
			"response_version", response.version,
			"error_code", detail.GetCode(),
		)

		return nil
	}

	if request.GetVersionInfo() != response.version {
		logger.V(1).Info("xDS feedback version does not match response", "result", "unmatched")
		return nil
	}

	logger.V(1).Info("xDS configuration acknowledged",
		"result", "ack",
		"response_version", response.version,
	)

	return nil
}

// OnStreamResponse 记录进入发送链路的响应，不承诺 gRPC 发送成功或 Envoy 已安装。
func (c *callbacks) OnStreamResponse(
	_ context.Context,
	streamID int64,
	_ *discoveryv3.DiscoveryRequest,
	response *discoveryv3.DiscoveryResponse,
) {
	if response.GetNonce() == "" || !supportedResourceType(response.GetTypeUrl()) {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if state := c.streams[streamID]; state != nil {
		// 每个活动流只保留四种已支持资源各自的最新响应；不持有 protobuf 指针。
		state.responses[response.GetTypeUrl()] = responseState{
			nonce:   response.GetNonce(),
			version: response.GetVersionInfo(),
		}
	}
}

func (c *callbacks) responseForRequest(
	streamID int64,
	request *discoveryv3.DiscoveryRequest,
) (string, responseState) {
	nodeID := request.GetNode().GetId()
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.streams[streamID]
	if state == nil {
		return nodeID, responseState{}
	}

	if nodeID != "" {
		state.nodeID = nodeID
	} else {
		nodeID = state.nodeID
	}

	return nodeID, state.responses[request.GetTypeUrl()]
}

func supportedResourceType(typeURL string) bool {
	switch typeURL {
	case resourcev3.ListenerType, resourcev3.RouteType, resourcev3.ClusterType, resourcev3.EndpointType:
		return true
	default:
		return false
	}
}

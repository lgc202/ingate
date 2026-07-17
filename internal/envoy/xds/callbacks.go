package xds

import (
	"context"
	"fmt"
	"sync"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	sotwv3 "github.com/envoyproxy/go-control-plane/pkg/server/sotw/v3"
)

type sentKey struct {
	streamID int64
	typeURL  string
}

type sentResponse struct {
	nodeID  string
	version string
	nonce   string
}

// Callbacks 记录 SotW stream、Node 和已发送响应，用 nonce 识别 ACK/NACK
//
// Callbacks 可被多个 SotW stream 并发调用
type Callbacks struct {
	sink EventSink

	mu          sync.Mutex
	sent        map[sentKey]sentResponse
	streamNodes map[int64]string
	nodeStreams map[string]int64
	// SotW request callback 不携带 context，因此只在 stream 生命周期内保留 OnStreamOpen 的 context
	streamContexts map[int64]context.Context
}

var _ sotwv3.Callbacks = (*Callbacks)(nil)

// NewCallbacks 创建 SotW callback registry
func NewCallbacks(sink EventSink) *Callbacks {
	return &Callbacks{
		sink:           sink,
		sent:           make(map[sentKey]sentResponse),
		streamNodes:    make(map[int64]string),
		nodeStreams:    make(map[string]int64),
		streamContexts: make(map[int64]context.Context),
	}
}

// OnStreamOpen 上报 SotW stream 建立事件
func (c *Callbacks) OnStreamOpen(ctx context.Context, streamID int64, typeURL string) error {
	c.mu.Lock()
	c.streamContexts[streamID] = ctx
	c.mu.Unlock()

	return c.sink(ctx, Event{
		Kind:     EventStreamOpened,
		StreamID: streamID,
		TypeURL:  typeURL,
	})
}

// OnStreamClosed 只根据本地 registry 清理 stream，不依赖外部传入的 Node
func (c *Callbacks) OnStreamClosed(streamID int64, _ *corev3.Node) {
	c.mu.Lock()
	nodeID := c.streamNodes[streamID]
	delete(c.streamNodes, streamID)
	delete(c.streamContexts, streamID)
	if nodeID != "" && c.nodeStreams[nodeID] == streamID {
		delete(c.nodeStreams, nodeID)
	}
	for key := range c.sent {
		if key.streamID == streamID {
			delete(c.sent, key)
		}
	}
	c.mu.Unlock()

	// SotW close callback 没有错误返回值，只能忽略 sink 返回错误
	_ = c.sink(context.Background(), Event{
		Kind:     EventStreamClosed,
		StreamID: streamID,
		NodeID:   nodeID,
	})
}

// OnStreamRequest 注册 Node，并按最新已发送 nonce 分类 ACK/NACK
func (c *Callbacks) OnStreamRequest(streamID int64, request *discoveryv3.DiscoveryRequest) error {
	nodeID := request.GetNode().GetId()
	if nodeID == "" {
		return fmt.Errorf("xDS stream %d requires a non-empty node ID", streamID)
	}

	c.mu.Lock()
	registeredNodeID, registered := c.streamNodes[streamID]
	if registered && registeredNodeID != nodeID {
		c.mu.Unlock()
		return fmt.Errorf("xDS stream %d is registered for node %q, got %q", streamID, registeredNodeID, nodeID)
	}
	if activeStreamID, active := c.nodeStreams[nodeID]; active && activeStreamID != streamID {
		c.mu.Unlock()
		return fmt.Errorf("xDS node %q is already connected on stream %d", nodeID, activeStreamID)
	}
	if !registered {
		c.streamNodes[streamID] = nodeID
		c.nodeStreams[nodeID] = streamID
	}

	key := sentKey{streamID: streamID, typeURL: request.GetTypeUrl()}
	sent, ok := c.sent[key]
	if !ok || request.GetResponseNonce() == "" || request.GetResponseNonce() != sent.nonce {
		c.mu.Unlock()
		return nil
	}
	delete(c.sent, key)
	ctx := c.streamContexts[streamID]
	c.mu.Unlock()

	event := Event{
		Kind:            EventACK,
		StreamID:        streamID,
		NodeID:          sent.nodeID,
		TypeURL:         request.GetTypeUrl(),
		Version:         sent.version,
		AcceptedVersion: request.GetVersionInfo(),
		Nonce:           sent.nonce,
	}
	if detail := request.GetErrorDetail(); detail != nil {
		event.Kind = EventNACK
		event.ErrorCode = detail.GetCode()
		event.ErrorMessage = detail.GetMessage()
	}

	if ctx == nil {
		ctx = context.Background()
	}
	return c.sink(ctx, event)
}

// OnStreamResponse 记录 go-control-plane 已生成的最终 nonce 和 version
func (c *Callbacks) OnStreamResponse(
	_ context.Context,
	streamID int64,
	_ *discoveryv3.DiscoveryRequest,
	response *discoveryv3.DiscoveryResponse,
) {
	c.mu.Lock()
	nodeID := c.streamNodes[streamID]
	streamCtx := c.streamContexts[streamID]
	sent := sentResponse{
		nodeID:  nodeID,
		version: response.GetVersionInfo(),
		nonce:   response.GetNonce(),
	}
	c.sent[sentKey{streamID: streamID, typeURL: response.GetTypeUrl()}] = sent
	c.mu.Unlock()

	if streamCtx == nil {
		streamCtx = context.Background()
	}
	// SotW response callback 没有错误返回值，只能忽略 sink 返回错误
	_ = c.sink(streamCtx, Event{
		Kind:     EventResponseSent,
		StreamID: streamID,
		NodeID:   nodeID,
		TypeURL:  response.GetTypeUrl(),
		Version:  sent.version,
		Nonce:    sent.nonce,
	})
}

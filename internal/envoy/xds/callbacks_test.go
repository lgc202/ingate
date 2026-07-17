package xds

import (
	"context"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

type callbackContextKey string

func TestOnStreamResponseUsesStreamContext(t *testing.T) {
	const streamID int64 = 7
	const contextKey callbackContextKey = "source"

	var responseEventContext context.Context
	callbacks := NewCallbacks(func(ctx context.Context, event Event) error {
		if event.Kind == EventResponseSent {
			responseEventContext = ctx
		}
		return nil
	})
	streamCtx := context.WithValue(context.Background(), contextKey, "stream")
	if err := callbacks.OnStreamOpen(streamCtx, streamID, resourcev3.ListenerType); err != nil {
		t.Fatalf("Callbacks.OnStreamOpen(%d) error = %v", streamID, err)
	}
	request := &discoveryv3.DiscoveryRequest{
		Node:    &corev3.Node{Id: "envoy-1"},
		TypeUrl: resourcev3.ListenerType,
	}
	if err := callbacks.OnStreamRequest(streamID, request); err != nil {
		t.Fatalf("Callbacks.OnStreamRequest(%d) error = %v", streamID, err)
	}

	responseCtx, cancel := context.WithCancel(context.WithValue(context.Background(), contextKey, "response"))
	cancel()
	callbacks.OnStreamResponse(responseCtx, streamID, request, &discoveryv3.DiscoveryResponse{
		TypeUrl:     resourcev3.ListenerType,
		VersionInfo: "version-1",
		Nonce:       "nonce-1",
	})

	if responseEventContext == nil {
		t.Fatal("Callbacks.OnStreamResponse() did not publish an EventResponseSent context")
	}
	if err := responseEventContext.Err(); err != nil {
		t.Errorf("Callbacks.OnStreamResponse() context error = %v, want nil", err)
	}
	if got := responseEventContext.Value(contextKey); got != "stream" {
		t.Errorf("Callbacks.OnStreamResponse() context value = %v, want stream context", got)
	}
}

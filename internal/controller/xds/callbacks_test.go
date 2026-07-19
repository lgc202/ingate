package xds

import (
	"context"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
)

type testContextKey string

func TestOnStreamResponseUsesStreamContext(t *testing.T) {
	const streamID int64 = 7
	const contextKey testContextKey = "source"

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

func TestOnStreamRequestReportsAcceptedVersionWithoutMatchingNonce(t *testing.T) {
	const streamID int64 = 8

	var observed *Event
	callbacks := NewCallbacks(func(_ context.Context, event Event) error {
		if event.Kind == EventAcceptedVersionObserved {
			copy := event
			observed = &copy
		}
		return nil
	})
	if err := callbacks.OnStreamOpen(context.Background(), streamID, resourcev3.ListenerType); err != nil {
		t.Fatalf("Callbacks.OnStreamOpen(%d) error = %v", streamID, err)
	}
	request := &discoveryv3.DiscoveryRequest{
		Node:        &corev3.Node{Id: "envoy-1"},
		TypeUrl:     resourcev3.ListenerType,
		VersionInfo: "version-1",
	}
	if err := callbacks.OnStreamRequest(streamID, request); err != nil {
		t.Fatalf("Callbacks.OnStreamRequest(%d) error = %v", streamID, err)
	}

	if observed == nil {
		t.Fatal("Callbacks.OnStreamRequest() did not report accepted version")
	}
	if observed.StreamID != streamID || observed.NodeID != "envoy-1" || observed.TypeURL != resourcev3.ListenerType {
		t.Errorf("accepted version observation identity = %#v, want stream %d node envoy-1 listener type", observed, streamID)
	}
	if observed.AcceptedVersion != request.VersionInfo {
		t.Errorf("accepted version observation = %q, want %q", observed.AcceptedVersion, request.VersionInfo)
	}
}

func TestOnStreamRequestDoesNotReportNACKVersionInfoAsAccepted(t *testing.T) {
	const streamID int64 = 9

	var observed, nacks int
	callbacks := NewCallbacks(func(_ context.Context, event Event) error {
		switch event.Kind {
		case EventAcceptedVersionObserved:
			observed++
		case EventNACK:
			nacks++
		}
		return nil
	})
	if err := callbacks.OnStreamOpen(context.Background(), streamID, resourcev3.ListenerType); err != nil {
		t.Fatalf("Callbacks.OnStreamOpen(%d) error = %v", streamID, err)
	}
	request := &discoveryv3.DiscoveryRequest{
		Node:    &corev3.Node{Id: "envoy-1"},
		TypeUrl: resourcev3.ListenerType,
	}
	if err := callbacks.OnStreamRequest(streamID, request); err != nil {
		t.Fatalf("Callbacks.OnStreamRequest(%d) registration error = %v", streamID, err)
	}
	callbacks.OnStreamResponse(context.Background(), streamID, request, &discoveryv3.DiscoveryResponse{
		TypeUrl:     resourcev3.ListenerType,
		VersionInfo: "candidate-version",
		Nonce:       "nonce-1",
	})

	nack := &discoveryv3.DiscoveryRequest{
		Node:          request.Node,
		TypeUrl:       resourcev3.ListenerType,
		VersionInfo:   "previous-version",
		ResponseNonce: "nonce-1",
		ErrorDetail:   &statuspb.Status{Code: 3, Message: "rejected"},
	}
	if err := callbacks.OnStreamRequest(streamID, nack); err != nil {
		t.Fatalf("Callbacks.OnStreamRequest(%d) NACK error = %v", streamID, err)
	}
	if observed != 0 {
		t.Errorf("Callbacks.OnStreamRequest(NACK) accepted observations = %d, want 0", observed)
	}
	if nacks != 1 {
		t.Errorf("Callbacks.OnStreamRequest(NACK) NACK events = %d, want 1", nacks)
	}
}

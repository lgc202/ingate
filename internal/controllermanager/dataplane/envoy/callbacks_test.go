package envoy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/go-logr/logr"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
)

func TestCallbacksFeedback(t *testing.T) {
	tests := []struct {
		name          string
		streamID      int64
		request       *discoveryv3.DiscoveryRequest
		wantResult    string
		wantVersion   string
		wantErrorCode int32
	}{
		{
			name:     "ACK",
			streamID: 1,
			request: &discoveryv3.DiscoveryRequest{
				TypeUrl:       resourcev3.ListenerType,
				ResponseNonce: "1",
				VersionInfo:   "v2",
			},
			wantResult:  "ack",
			wantVersion: "v2",
		},
		{
			name:     "NACK identifies rejected version, not last accepted version",
			streamID: 1,
			request: &discoveryv3.DiscoveryRequest{
				TypeUrl:       resourcev3.ListenerType,
				ResponseNonce: "1",
				VersionInfo:   "v1",
				ErrorDetail:   &statuspb.Status{Code: 3, Message: "secret configuration"},
			},
			wantResult:    "nack",
			wantVersion:   "v2",
			wantErrorCode: 3,
		},
		{
			name:     "NACK before any accepted version",
			streamID: 1,
			request: &discoveryv3.DiscoveryRequest{
				TypeUrl:       resourcev3.ListenerType,
				ResponseNonce: "1",
				ErrorDetail:   &statuspb.Status{Code: 3},
			},
			wantResult:    "nack",
			wantVersion:   "v2",
			wantErrorCode: 3,
		},
		{
			name:     "present error detail is NACK even with zero code",
			streamID: 1,
			request: &discoveryv3.DiscoveryRequest{
				TypeUrl:       resourcev3.ListenerType,
				ResponseNonce: "1",
				VersionInfo:   "v2",
				ErrorDetail:   &statuspb.Status{},
			},
			wantResult:  "nack",
			wantVersion: "v2",
		},
		{
			name:     "stale nonce",
			streamID: 1,
			request: &discoveryv3.DiscoveryRequest{
				TypeUrl:       resourcev3.ListenerType,
				ResponseNonce: "old",
				VersionInfo:   "v2",
			},
			wantResult: "unmatched",
		},
		{
			name:     "stale NACK cannot reject latest response",
			streamID: 1,
			request: &discoveryv3.DiscoveryRequest{
				TypeUrl:       resourcev3.ListenerType,
				ResponseNonce: "old",
				ErrorDetail:   &statuspb.Status{Code: 3},
			},
			wantResult: "unmatched",
		},
		{
			name:     "nonce from another resource type",
			streamID: 1,
			request: &discoveryv3.DiscoveryRequest{
				TypeUrl:       resourcev3.RouteType,
				ResponseNonce: "1",
				VersionInfo:   "v2",
			},
			wantResult: "unmatched",
		},
		{
			name:     "nonce from another stream",
			streamID: 2,
			request: &discoveryv3.DiscoveryRequest{
				TypeUrl:       resourcev3.ListenerType,
				ResponseNonce: "1",
				VersionInfo:   "v2",
			},
			wantResult: "unmatched",
		},
		{
			name:     "ACK with mismatched version",
			streamID: 1,
			request: &discoveryv3.DiscoveryRequest{
				TypeUrl:       resourcev3.ListenerType,
				ResponseNonce: "1",
				VersionInfo:   "v1",
			},
			wantResult: "unmatched",
		},
		{
			name:     "initial subscription is not ACK",
			streamID: 1,
			request:  &discoveryv3.DiscoveryRequest{TypeUrl: resourcev3.ListenerType},
		},
		{
			name:     "unsupported resource type",
			streamID: 1,
			request: &discoveryv3.DiscoveryRequest{
				TypeUrl:       "unknown",
				ResponseNonce: "1",
				VersionInfo:   "v2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callback, logs := newTestCallbacks(t)
			if err := callback.OnStreamOpen(t.Context(), 1, resourcev3.AnyType); err != nil {
				t.Fatalf("OnStreamOpen() = %v", err)
			}

			if err := callback.OnStreamRequest(1, &discoveryv3.DiscoveryRequest{
				Node:    &corev3.Node{Id: "envoy-1"},
				TypeUrl: resourcev3.ListenerType,
			}); err != nil {
				t.Fatalf("OnStreamRequest(subscription) = %v", err)
			}

			callback.OnStreamResponse(t.Context(), 1, nil, &discoveryv3.DiscoveryResponse{
				TypeUrl:     resourcev3.ListenerType,
				Nonce:       "old",
				VersionInfo: "v1",
			})
			callback.OnStreamResponse(t.Context(), 1, nil, &discoveryv3.DiscoveryResponse{
				TypeUrl:     resourcev3.ListenerType,
				Nonce:       "1",
				VersionInfo: "v2",
			})
			if err := callback.OnStreamRequest(tt.streamID, tt.request); err != nil {
				t.Fatalf("OnStreamRequest(feedback) = %v, want nil", err)
			}

			if tt.wantResult == "" {
				if logs.Len() != 0 {
					t.Fatalf("unexpected feedback log: %s", logs.String())
				}

				return
			}

			record := readCallbackLog(t, logs)
			if got := record["result"]; got != tt.wantResult {
				t.Errorf("result = %v, want %q", got, tt.wantResult)
			}

			if got, exists := record["response_version"]; tt.wantVersion == "" {
				if exists {
					t.Errorf("unmatched feedback has response_version = %v", got)
				}
			} else if got != tt.wantVersion {
				t.Errorf("response_version = %v, want %q", got, tt.wantVersion)
			}

			if got := record["client_version"]; got != tt.request.GetVersionInfo() {
				t.Errorf("client_version = %v, want %q", got, tt.request.GetVersionInfo())
			}

			if tt.streamID == 1 && record["node_id"] != "envoy-1" {
				t.Errorf("node_id = %v, want cached envoy-1", record["node_id"])
			}

			if tt.wantResult == "nack" && record["error_code"] != float64(tt.wantErrorCode) {
				t.Errorf("error_code = %v, want %d", record["error_code"], tt.wantErrorCode)
			}

			if strings.Contains(logs.String(), "secret configuration") {
				t.Error("log exposes ErrorDetail.Message")
			}
		})
	}
}

func TestCallbacksStreamIsolationAndCleanup(t *testing.T) {
	callback, logs := newTestCallbacks(t)
	versions := map[int64]string{1: "v1", 2: "v2"}
	for streamID, version := range versions {
		if err := callback.OnStreamOpen(t.Context(), streamID, resourcev3.AnyType); err != nil {
			t.Fatalf("OnStreamOpen(%d) = %v", streamID, err)
		}

		callback.OnStreamResponse(t.Context(), streamID, nil, &discoveryv3.DiscoveryResponse{
			TypeUrl:     resourcev3.ClusterType,
			Nonce:       "1",
			VersionInfo: version,
		})
	}

	for streamID, version := range versions {
		if err := callback.OnStreamRequest(streamID, &discoveryv3.DiscoveryRequest{
			TypeUrl:       resourcev3.ClusterType,
			ResponseNonce: "1",
			VersionInfo:   version,
		}); err != nil {
			t.Fatalf("OnStreamRequest(%d) = %v", streamID, err)
		}

		record := readCallbackLog(t, logs)
		if record["result"] != "ack" || record["response_version"] != version {
			t.Errorf("stream %d feedback = %v, want ACK of %s", streamID, record, version)
		}

		logs.Reset()
	}

	callback.OnStreamClosed(1, nil)
	callback.OnStreamResponse(t.Context(), 1, nil, &discoveryv3.DiscoveryResponse{
		TypeUrl:     resourcev3.ClusterType,
		Nonce:       "2",
		VersionInfo: "late",
	})
	if _, exists := callback.streams[1]; exists {
		t.Fatal("late response recreated closed stream")
	}

	if len(callback.streams) != 1 {
		t.Fatalf("active streams = %d, want remaining stream only", len(callback.streams))
	}

	if err := callback.OnStreamOpen(t.Context(), 3, resourcev3.AnyType); err != nil {
		t.Fatalf("OnStreamOpen(reconnect) = %v", err)
	}

	if err := callback.OnStreamRequest(3, &discoveryv3.DiscoveryRequest{
		TypeUrl:       resourcev3.ClusterType,
		ResponseNonce: "1",
		VersionInfo:   "v1",
	}); err != nil {
		t.Fatalf("OnStreamRequest(reconnect) = %v", err)
	}

	record := readCallbackLog(t, logs)
	if record["result"] != "unmatched" || record["response_version"] != nil {
		t.Errorf("reconnected feedback = %v, want unmatched without response version", record)
	}

	callback.OnStreamClosed(2, nil)
	callback.OnStreamClosed(3, nil)
	if len(callback.streams) != 0 {
		t.Errorf("closed streams retained: %d", len(callback.streams))
	}
}

func TestCallbacksResponseStateBounded(t *testing.T) {
	callback := newCallbacks(logr.Discard())
	if err := callback.OnStreamOpen(t.Context(), 1, resourcev3.AnyType); err != nil {
		t.Fatalf("OnStreamOpen() = %v", err)
	}

	for _, typeURL := range []string{
		resourcev3.ListenerType, resourcev3.RouteType, resourcev3.ClusterType, resourcev3.EndpointType,
	} {
		for i := range 100 {
			response := &discoveryv3.DiscoveryResponse{
				TypeUrl:     typeURL,
				Nonce:       strconv.Itoa(i),
				VersionInfo: "v2",
			}
			callback.OnStreamResponse(t.Context(), 1, nil, response)
			response.Nonce = "mutated"
			response.VersionInfo = "mutated"
			callback.OnStreamResponse(t.Context(), 1, nil, &discoveryv3.DiscoveryResponse{
				TypeUrl:     "unknown/" + strings.Repeat("x", i),
				Nonce:       "1",
				VersionInfo: "v2",
			})
		}

		if got := callback.streams[1].responses[typeURL]; got.nonce != "99" || got.version != "v2" {
			t.Errorf("stored response for %s = %+v, want copied 99/v2", typeURL, got)
		}
	}

	callback.OnStreamResponse(t.Context(), 1, nil, &discoveryv3.DiscoveryResponse{
		TypeUrl: resourcev3.ListenerType, VersionInfo: "without-nonce",
	})
	if got := len(callback.streams[1].responses); got != 4 {
		t.Errorf("stored response types = %d, want 4", got)
	}

	if got := callback.streams[1].responses[resourcev3.ListenerType].version; got != "v2" {
		t.Errorf("empty nonce replaced response version with %q", got)
	}
}

func TestCallbacksConcurrentStreams(t *testing.T) {
	callback := newCallbacks(logr.Discard())
	var workers sync.WaitGroup
	for streamID := int64(1); streamID <= 32; streamID++ {
		workers.Go(func() {
			if err := callback.OnStreamOpen(context.Background(), streamID, resourcev3.AnyType); err != nil {
				t.Errorf("OnStreamOpen(%d) = %v", streamID, err)
				return
			}

			defer callback.OnStreamClosed(streamID, nil)
			for range 100 {
				callback.OnStreamResponse(context.Background(), streamID, nil, &discoveryv3.DiscoveryResponse{
					TypeUrl:     resourcev3.ListenerType,
					Nonce:       "1",
					VersionInfo: "v1",
				})
				if err := callback.OnStreamRequest(streamID, &discoveryv3.DiscoveryRequest{
					TypeUrl:       resourcev3.ListenerType,
					ResponseNonce: "1",
					VersionInfo:   "v1",
				}); err != nil {
					t.Errorf("OnStreamRequest(%d) = %v", streamID, err)
					return
				}
			}
		})
	}

	workers.Wait()
	if len(callback.streams) != 0 {
		t.Errorf("active streams after workers stopped = %d, want 0", len(callback.streams))
	}
}

func newTestCallbacks(t *testing.T) (*callbacks, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	logger := logr.FromSlogHandler(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	return newCallbacks(logger), &logs
}

func readCallbackLog(t *testing.T, logs *bytes.Buffer) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(logs.Bytes()))
	var record map[string]any
	if err := decoder.Decode(&record); err != nil {
		t.Fatalf("decode callback log: %v; logs = %s", err, logs.String())
	}

	var extra map[string]any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("expected exactly one callback log, extra = %v, error = %v", extra, err)
	}

	return record
}

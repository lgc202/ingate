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
		request       *discoveryv3.DeltaDiscoveryRequest
		wantResult    string
		wantVersion   string
		wantErrorCode int32
	}{
		{
			name:     "ACK",
			streamID: 1,
			request: &discoveryv3.DeltaDiscoveryRequest{
				TypeUrl:       resourcev3.ListenerType,
				ResponseNonce: "1",
			},
			wantResult:  "ack",
			wantVersion: "v2",
		},
		{
			name:     "NACK identifies rejected response",
			streamID: 1,
			request: &discoveryv3.DeltaDiscoveryRequest{
				TypeUrl:       resourcev3.ListenerType,
				ResponseNonce: "1",
				ErrorDetail:   &statuspb.Status{Code: 3, Message: "secret configuration"},
			},
			wantResult:    "nack",
			wantVersion:   "v2",
			wantErrorCode: 3,
		},
		{
			name:     "present error detail is NACK even with zero code",
			streamID: 1,
			request: &discoveryv3.DeltaDiscoveryRequest{
				TypeUrl:       resourcev3.ListenerType,
				ResponseNonce: "1",
				ErrorDetail:   &statuspb.Status{},
			},
			wantResult:  "nack",
			wantVersion: "v2",
		},
		{
			name:     "stale nonce",
			streamID: 1,
			request: &discoveryv3.DeltaDiscoveryRequest{
				TypeUrl:       resourcev3.ListenerType,
				ResponseNonce: "old",
			},
			wantResult: "unmatched",
		},
		{
			name:     "stale NACK cannot reject latest response",
			streamID: 1,
			request: &discoveryv3.DeltaDiscoveryRequest{
				TypeUrl:       resourcev3.ListenerType,
				ResponseNonce: "old",
				ErrorDetail:   &statuspb.Status{Code: 3},
			},
			wantResult: "unmatched",
		},
		{
			name:     "nonce from another resource type",
			streamID: 1,
			request: &discoveryv3.DeltaDiscoveryRequest{
				TypeUrl:       resourcev3.RouteType,
				ResponseNonce: "1",
			},
			wantResult: "unmatched",
		},
		{
			name:     "nonce from another stream",
			streamID: 2,
			request: &discoveryv3.DeltaDiscoveryRequest{
				TypeUrl:       resourcev3.ListenerType,
				ResponseNonce: "1",
			},
			wantResult: "unmatched",
		},
		{
			name:     "initial subscription is not ACK",
			streamID: 1,
			request:  &discoveryv3.DeltaDiscoveryRequest{TypeUrl: resourcev3.ListenerType},
		},
		{
			name:     "unsupported resource type",
			streamID: 1,
			request: &discoveryv3.DeltaDiscoveryRequest{
				TypeUrl:       "unknown",
				ResponseNonce: "1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callback, logs := newTestCallbacks(t)
			if err := callback.OnDeltaStreamOpen(t.Context(), 1, resourcev3.AnyType); err != nil {
				t.Fatalf("OnDeltaStreamOpen() = %v", err)
			}

			if err := callback.OnStreamDeltaRequest(1, &discoveryv3.DeltaDiscoveryRequest{
				Node:    &corev3.Node{Id: "envoy-1"},
				TypeUrl: resourcev3.ListenerType,
			}); err != nil {
				t.Fatalf("OnStreamDeltaRequest(subscription) = %v", err)
			}

			callback.OnStreamDeltaResponse(1, nil, &discoveryv3.DeltaDiscoveryResponse{
				TypeUrl:           resourcev3.ListenerType,
				Nonce:             "old",
				SystemVersionInfo: "v1",
			})
			callback.OnStreamDeltaResponse(1, nil, &discoveryv3.DeltaDiscoveryResponse{
				TypeUrl:           resourcev3.ListenerType,
				Nonce:             "1",
				SystemVersionInfo: "v2",
			})
			if err := callback.OnStreamDeltaRequest(tt.streamID, tt.request); err != nil {
				t.Fatalf("OnStreamDeltaRequest(feedback) = %v, want nil", err)
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

			if got, exists := record["system_version"]; tt.wantVersion == "" {
				if exists {
					t.Errorf("unmatched feedback has system_version = %v", got)
				}
			} else if got != tt.wantVersion {
				t.Errorf("system_version = %v, want %q", got, tt.wantVersion)
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

func TestCallbacksACKWithoutDiagnosticVersion(t *testing.T) {
	callback, logs := newTestCallbacks(t)
	if err := callback.OnDeltaStreamOpen(t.Context(), 1, resourcev3.AnyType); err != nil {
		t.Fatalf("OnDeltaStreamOpen() error = %v", err)
	}

	// system_version_info 是可选诊断字段，不能要求客户端用它确认响应。
	callback.OnStreamDeltaResponse(1, nil, &discoveryv3.DeltaDiscoveryResponse{
		TypeUrl: resourcev3.ListenerType, Nonce: "1",
	})

	if err := callback.OnStreamDeltaRequest(1, &discoveryv3.DeltaDiscoveryRequest{
		TypeUrl: resourcev3.ListenerType, ResponseNonce: "1",
	}); err != nil {
		t.Fatalf("OnStreamDeltaRequest(ACK without version) error = %v", err)
	}

	record := readCallbackLog(t, logs)
	if record["result"] != "ack" || record["system_version"] != "" {
		t.Errorf("OnStreamDeltaRequest(ACK without version) log = %v, want ack and empty diagnostic version", record)
	}
}

func TestCallbacksStreamIsolationAndCleanup(t *testing.T) {
	callback, logs := newTestCallbacks(t)
	versions := map[int64]string{1: "v1", 2: "v2"}
	for streamID, version := range versions {
		if err := callback.OnDeltaStreamOpen(t.Context(), streamID, resourcev3.AnyType); err != nil {
			t.Fatalf("OnDeltaStreamOpen(%d) = %v", streamID, err)
		}

		callback.OnStreamDeltaResponse(streamID, nil, &discoveryv3.DeltaDiscoveryResponse{
			TypeUrl:           resourcev3.ClusterType,
			Nonce:             "1",
			SystemVersionInfo: version,
		})
	}

	for streamID, version := range versions {
		if err := callback.OnStreamDeltaRequest(streamID, &discoveryv3.DeltaDiscoveryRequest{
			TypeUrl:       resourcev3.ClusterType,
			ResponseNonce: "1",
		}); err != nil {
			t.Fatalf("OnStreamDeltaRequest(%d) = %v", streamID, err)
		}

		record := readCallbackLog(t, logs)
		if record["result"] != "ack" || record["system_version"] != version {
			t.Errorf("stream %d feedback = %v, want ACK of %s", streamID, record, version)
		}

		logs.Reset()
	}

	callback.OnDeltaStreamClosed(1, nil)
	callback.OnStreamDeltaResponse(1, nil, &discoveryv3.DeltaDiscoveryResponse{
		TypeUrl:           resourcev3.ClusterType,
		Nonce:             "2",
		SystemVersionInfo: "late",
	})
	if _, exists := callback.streams[1]; exists {
		t.Fatal("late response recreated closed stream")
	}

	if len(callback.streams) != 1 {
		t.Fatalf("active streams = %d, want remaining stream only", len(callback.streams))
	}

	if err := callback.OnDeltaStreamOpen(t.Context(), 3, resourcev3.AnyType); err != nil {
		t.Fatalf("OnDeltaStreamOpen(reconnect) = %v", err)
	}

	if err := callback.OnStreamDeltaRequest(3, &discoveryv3.DeltaDiscoveryRequest{
		TypeUrl:       resourcev3.ClusterType,
		ResponseNonce: "1",
	}); err != nil {
		t.Fatalf("OnStreamDeltaRequest(reconnect) = %v", err)
	}

	record := readCallbackLog(t, logs)
	if record["result"] != "unmatched" || record["system_version"] != nil {
		t.Errorf("reconnected feedback = %v, want unmatched without response version", record)
	}

	callback.OnDeltaStreamClosed(2, nil)
	callback.OnDeltaStreamClosed(3, nil)
	if len(callback.streams) != 0 {
		t.Errorf("closed streams retained: %d", len(callback.streams))
	}
}

func TestCallbacksResponseStateBounded(t *testing.T) {
	callback := newCallbacks(logr.Discard())
	if err := callback.OnDeltaStreamOpen(t.Context(), 1, resourcev3.AnyType); err != nil {
		t.Fatalf("OnDeltaStreamOpen() = %v", err)
	}

	for _, typeURL := range []string{
		resourcev3.ListenerType, resourcev3.RouteType, resourcev3.ClusterType, resourcev3.EndpointType,
	} {
		for i := range 100 {
			response := &discoveryv3.DeltaDiscoveryResponse{
				TypeUrl:           typeURL,
				Nonce:             strconv.Itoa(i),
				SystemVersionInfo: "v2",
			}
			callback.OnStreamDeltaResponse(1, nil, response)
			response.Nonce = "mutated"
			response.SystemVersionInfo = "mutated"
			callback.OnStreamDeltaResponse(1, nil, &discoveryv3.DeltaDiscoveryResponse{
				TypeUrl:           "unknown/" + strings.Repeat("x", i),
				Nonce:             "1",
				SystemVersionInfo: "v2",
			})
		}

		if got := callback.streams[1].responses[typeURL]; got.nonce != "99" || got.systemVersion != "v2" {
			t.Errorf("stored response for %s = %+v, want copied 99/v2", typeURL, got)
		}
	}

	callback.OnStreamDeltaResponse(1, nil, &discoveryv3.DeltaDiscoveryResponse{
		TypeUrl: resourcev3.ListenerType, SystemVersionInfo: "without-nonce",
	})
	if got := len(callback.streams[1].responses); got != 4 {
		t.Errorf("stored response types = %d, want 4", got)
	}

	if got := callback.streams[1].responses[resourcev3.ListenerType].systemVersion; got != "v2" {
		t.Errorf("empty nonce replaced response version with %q", got)
	}
}

func TestCallbacksConcurrentStreams(t *testing.T) {
	callback := newCallbacks(logr.Discard())
	var workers sync.WaitGroup
	for streamID := int64(1); streamID <= 32; streamID++ {
		workers.Go(func() {
			if err := callback.OnDeltaStreamOpen(context.Background(), streamID, resourcev3.AnyType); err != nil {
				t.Errorf("OnDeltaStreamOpen(%d) = %v", streamID, err)
				return
			}

			defer callback.OnDeltaStreamClosed(streamID, nil)
			for range 100 {
				callback.OnStreamDeltaResponse(streamID, nil, &discoveryv3.DeltaDiscoveryResponse{
					TypeUrl:           resourcev3.ListenerType,
					Nonce:             "1",
					SystemVersionInfo: "v1",
				})
				if err := callback.OnStreamDeltaRequest(streamID, &discoveryv3.DeltaDiscoveryRequest{
					TypeUrl:       resourcev3.ListenerType,
					ResponseNonce: "1",
				}); err != nil {
					t.Errorf("OnStreamDeltaRequest(%d) = %v", streamID, err)
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

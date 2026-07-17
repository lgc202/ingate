package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	controllerclient "github.com/lgc202/ingate/internal/adminapi/client/controller"
	systemstatusdto "github.com/lgc202/ingate/internal/adminapi/handler/systemstatus/dto"
)

func TestSystemStatus(t *testing.T) {
	controllerServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/internal/v1/status" {
			t.Errorf("request path = %q, want %q", request.URL.Path, "/internal/v1/status")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{
			"candidateVersion":"ingate/candidate",
			"activeVersion":"ingate/active",
			"configReady":true,
			"deliveryState":"WaitingForACK",
			"connectedEnvoys":2,
			"acks":{"required":4,"received":3},
			"nacks":{"count":1},
			"lastNack":{"nodeID":"envoy-1","typeURL":"type.googleapis.com/envoy.config.route.v3.RouteConfiguration","version":"ingate/candidate","time":"2026-07-17T10:20:30Z","message":"route rejected"},
			"ackTimedOut":false,
			"reconciled":true,
			"diagnostics":[{"message":"internal detail"}]
		}`)
	}))
	t.Cleanup(controllerServer.Close)

	body := requestSystemStatus(t, controllerServer.URL, 500*time.Millisecond)
	if body.Code != http.StatusOK || body.Msg != "ok" {
		t.Fatalf("response wrapper = {code:%d msg:%q}, want {code:200 msg:%q}", body.Code, body.Msg, "ok")
	}
	status := body.Data
	if !status.Available || !status.ConfigReady || status.DeliveryState != "WaitingForACK" {
		t.Fatalf("status core fields = %#v", status)
	}
	if status.CandidateVersion != "ingate/candidate" || status.ActiveVersion != "ingate/active" {
		t.Fatalf("status versions = candidate %q active %q", status.CandidateVersion, status.ActiveVersion)
	}
	if status.ConnectedEnvoys != 2 || status.ACK.Required != 4 || status.ACK.Received != 3 {
		t.Fatalf("status delivery progress = envoys %d ack %#v", status.ConnectedEnvoys, status.ACK)
	}
	if status.LastNACK == nil || status.LastNACK.NodeID != "envoy-1" || status.LastNACK.Message != "route rejected" {
		t.Fatalf("status last NACK = %#v", status.LastNACK)
	}

	for _, field := range []string{"diagnostics", "reconciled", "nacks", "ackTimedOut"} {
		if _, ok := body.RawData[field]; ok {
			t.Errorf("product response exposes internal field %q", field)
		}
	}
}

func TestSystemStatusUnavailable(t *testing.T) {
	tests := []struct {
		name          string
		handler       http.HandlerFunc
		closeBefore   bool
		clientTimeout time.Duration
	}{
		{
			name:        "controller unavailable",
			handler:     func(http.ResponseWriter, *http.Request) {},
			closeBefore: true,
		},
		{
			name: "timeout",
			handler: func(_ http.ResponseWriter, request *http.Request) {
				<-request.Context().Done()
			},
			clientTimeout: 20 * time.Millisecond,
		},
		{
			name: "non 2xx",
			handler: func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusServiceUnavailable)
			},
		},
		{
			name: "bad JSON",
			handler: func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(writer, `{"configReady":`)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controllerServer := httptest.NewServer(test.handler)
			controllerURL := controllerServer.URL
			if test.closeBefore {
				controllerServer.Close()
			} else {
				t.Cleanup(controllerServer.Close)
			}

			timeout := test.clientTimeout
			if timeout == 0 {
				timeout = 500 * time.Millisecond
			}
			body := requestSystemStatus(t, controllerURL, timeout)
			if body.Code != http.StatusOK || body.Msg != "ok" {
				t.Fatalf("response wrapper = {code:%d msg:%q}, want {code:200 msg:%q}", body.Code, body.Msg, "ok")
			}
			if body.Data.Available {
				t.Fatalf("available = true, want false")
			}
			if body.Data.Message != "暂时无法获取控制器运行状态，请稍后重试" {
				t.Fatalf("message = %q", body.Data.Message)
			}
			if body.Data.DeliveryState != "NoConfig" || body.Data.ConnectedEnvoys != 0 {
				t.Fatalf("fallback status = %#v", body.Data)
			}
		})
	}
}

type systemStatusResponse struct {
	Code    int                                 `json:"code"`
	Msg     string                              `json:"msg"`
	Data    systemstatusdto.GetSystemStatusResp `json:"data"`
	RawData map[string]json.RawMessage          `json:"-"`
}

func requestSystemStatus(t *testing.T, controllerURL string, timeout time.Duration) systemStatusResponse {
	t.Helper()

	controllerStatusClient, err := controllerclient.New(controllerURL, timeout)
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := New(nil, controllerStatusClient, "", "", logger).router()

	request := httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want %d", recorder.Code, http.StatusOK)
	}

	responseBody := recorder.Body.Bytes()
	var body systemStatusResponse
	if err := json.Unmarshal(responseBody, &body); err != nil {
		t.Fatalf("decode response %q: %v", strings.TrimSpace(string(responseBody)), err)
	}
	var raw struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &raw); err != nil {
		t.Fatalf("decode raw response: %v", err)
	}
	body.RawData = raw.Data
	return body
}

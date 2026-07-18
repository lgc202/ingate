package wasm

import (
	"encoding/json"
	"strings"
	"testing"

	config "github.com/lgc202/ingate/pkg/plugin/aiproxy"
	"github.com/lgc202/ingate/plugins/aiproxy/internal/policy"
	aiproxyruntime "github.com/lgc202/ingate/plugins/aiproxy/internal/runtime"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

func TestHTTPContextRoutesAndRewritesOpenAIRequest(t *testing.T) {
	host, contextID := newTestHost(t, modelConfig("secret"), aiRouteName("config-1"))
	headers := [][2]string{
		{":method", "POST"},
		{":path", "/v1/chat/completions"},
		{"content-type", "application/json"},
		{"content-length", "90"},
		{"authorization", "Bearer client-token"},
	}

	if action := host.CallOnRequestHeaders(contextID, headers, false); action != types.ActionPause {
		t.Fatalf("OnHttpRequestHeaders(valid AI request) action = %v, want pause", action)
	}
	for _, name := range []string{"authorization", "content-length"} {
		if values := headerValues(host.GetCurrentRequestHeaders(contextID), name); len(values) != 0 {
			t.Errorf("OnHttpRequestHeaders(valid AI request) header %q = %v, want removed", name, values)
		}
	}

	body := []byte(`{"model":"assistant","stream":true,"messages":[{"role":"user","content":"hello"}]}`)
	if action := host.CallOnRequestBody(contextID, body, true); action != types.ActionContinue {
		t.Fatalf("OnHttpRequestBody(valid AI request) action = %v, want continue", action)
	}
	requestHeaders := host.GetCurrentRequestHeaders(contextID)
	if got := headerValues(requestHeaders, "authorization"); len(got) != 1 || got[0] != "Bearer secret" {
		t.Errorf("OnHttpRequestBody(valid AI request) authorization headers = %v, want [Bearer secret]", got)
	}
	var rewritten map[string]any
	if err := json.Unmarshal(host.GetCurrentRequestBody(contextID), &rewritten); err != nil {
		t.Fatalf("json.Unmarshal(rewritten request body) error = %v", err)
	}
	if rewritten["model"] != "gpt-4o-mini" || rewritten["stream"] != true {
		t.Errorf("rewritten request body = %v, want model gpt-4o-mini and stream true", rewritten)
	}

	responseHeaders := [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}
	if action := host.CallOnResponseHeaders(contextID, responseHeaders, false); action != types.ActionContinue {
		t.Errorf("OnHttpResponseHeaders(SSE) action = %v, want continue", action)
	}
	chunk := []byte("data: {\"id\":\"chunk-1\"}\n\n")
	if action := host.CallOnResponseBody(contextID, chunk, false); action != types.ActionContinue {
		t.Errorf("OnHttpResponseBody(SSE) action = %v, want continue", action)
	}
	if got := string(host.GetCurrentResponseBody(contextID)); got != string(chunk) {
		t.Errorf("OnHttpResponseBody(SSE) body = %q, want unchanged %q", got, chunk)
	}
}

func TestHTTPContextDoesNotForwardClientAuthorizationWithoutAPIKey(t *testing.T) {
	host, contextID := newTestHost(t, modelConfig(""), aiRouteName("config-1"))
	headers := [][2]string{
		{":method", "POST"},
		{":path", "/v1/chat/completions"},
		{"authorization", "Bearer client-token"},
	}

	if action := host.CallOnRequestHeaders(contextID, headers, false); action != types.ActionPause {
		t.Fatalf("OnHttpRequestHeaders(API key omitted) action = %v, want pause", action)
	}
	if action := host.CallOnRequestBody(contextID, []byte(`{"model":"assistant","messages":[]}`), true); action != types.ActionContinue {
		t.Fatalf("OnHttpRequestBody(API key omitted) action = %v, want continue", action)
	}
	if got := headerValues(host.GetCurrentRequestHeaders(contextID), "authorization"); len(got) != 0 {
		t.Errorf("OnHttpRequestBody(API key omitted) authorization headers = %v, want none", got)
	}
}

func TestHTTPContextPassesThroughOrdinaryResponse(t *testing.T) {
	host, contextID := newTestHost(t, modelConfig("secret"), aiRouteName("config-1"))
	requestHeaders := [][2]string{{":method", "POST"}, {":path", "/v1/chat/completions"}}

	if action := host.CallOnRequestHeaders(contextID, requestHeaders, false); action != types.ActionPause {
		t.Fatalf("OnHttpRequestHeaders(ordinary response) action = %v, want pause", action)
	}
	if action := host.CallOnRequestBody(contextID, []byte(`{"model":"assistant","messages":[]}`), true); action != types.ActionContinue {
		t.Fatalf("OnHttpRequestBody(ordinary response) action = %v, want continue", action)
	}

	responseHeaders := [][2]string{{":status", "200"}, {"content-type", "application/json"}}
	if action := host.CallOnResponseHeaders(contextID, responseHeaders, false); action != types.ActionContinue {
		t.Errorf("OnHttpResponseHeaders(ordinary response) action = %v, want continue", action)
	}
	body := []byte(`{"id":"chatcmpl-1","choices":[]}`)
	if action := host.CallOnResponseBody(contextID, body, true); action != types.ActionContinue {
		t.Errorf("OnHttpResponseBody(ordinary response) action = %v, want continue", action)
	}
	if got := string(host.GetCurrentResponseBody(contextID)); got != string(body) {
		t.Errorf("OnHttpResponseBody(ordinary response) body = %q, want unchanged %q", got, body)
	}
}

func TestHTTPContextPreservesHeadersOutsideConfiguredRoute(t *testing.T) {
	host, contextID := newTestHost(t, modelConfig("secret"), "ingate-route/gateway-1/route-1/ordinary/POST")
	headers := [][2]string{
		{":method", "POST"},
		{":path", "/ordinary"},
		{"authorization", "Bearer client-token"},
		{"content-length", "12"},
	}

	if action := host.CallOnRequestHeaders(contextID, headers, true); action != types.ActionContinue {
		t.Fatalf("OnHttpRequestHeaders(unconfigured route) action = %v, want continue", action)
	}
	requestHeaders := host.GetCurrentRequestHeaders(contextID)
	if got := headerValues(requestHeaders, "authorization"); len(got) != 1 || got[0] != "Bearer client-token" {
		t.Errorf("OnHttpRequestHeaders(unconfigured route) authorization headers = %v, want client authorization preserved", got)
	}
	if got := headerValues(requestHeaders, "content-length"); len(got) != 1 || got[0] != "12" {
		t.Errorf("OnHttpRequestHeaders(unconfigured route) content-length headers = %v, want preserved", got)
	}
}

func TestHTTPContextFailsClosedWhenAIRouteConfigIsStale(t *testing.T) {
	host, contextID := newTestHost(t, modelConfig("secret"), aiRouteName("config-2"))
	headers := [][2]string{
		{":method", "POST"},
		{":path", "/v1/chat/completions"},
		{"authorization", "Bearer client-token"},
	}

	if action := host.CallOnRequestHeaders(contextID, headers, false); action != types.ActionPause {
		t.Fatalf("OnHttpRequestHeaders(stale AI config) action = %v, want pause", action)
	}
	if values := headerValues(host.GetCurrentRequestHeaders(contextID), "authorization"); len(values) != 0 {
		t.Errorf("OnHttpRequestHeaders(stale AI config) authorization headers = %v, want removed", values)
	}
	response := host.GetSentLocalResponse(contextID)
	if response == nil {
		t.Fatal("OnHttpRequestHeaders(stale AI config) local response = nil, want response")
	}
	if response.StatusCode != 500 {
		t.Errorf("OnHttpRequestHeaders(stale AI config) status = %d, want 500", response.StatusCode)
	}
}

func TestHTTPContextReturnsOpenAIErrorForInvalidJSON(t *testing.T) {
	host, contextID := newTestHost(t, modelConfig("secret"), aiRouteName("config-1"))
	headers := [][2]string{{":method", "POST"}, {":path", "/v1/chat/completions"}}

	if action := host.CallOnRequestHeaders(contextID, headers, false); action != types.ActionPause {
		t.Fatalf("OnHttpRequestHeaders(invalid JSON) action = %v, want pause", action)
	}
	if action := host.CallOnRequestBody(contextID, []byte(`{"model":`), true); action != types.ActionPause {
		t.Fatalf("OnHttpRequestBody(invalid JSON) action = %v, want pause", action)
	}
	response := host.GetSentLocalResponse(contextID)
	if response == nil {
		t.Fatal("OnHttpRequestBody(invalid JSON) local response = nil, want response")
	}
	if response.StatusCode != 400 {
		t.Errorf("OnHttpRequestBody(invalid JSON) status = %d, want 400", response.StatusCode)
	}
	if body := string(response.Data); !strings.Contains(body, `"error"`) || !strings.Contains(body, `"invalid_json"`) {
		t.Errorf("OnHttpRequestBody(invalid JSON) body = %q, want OpenAI-compatible invalid_json error", body)
	}
}

func TestHTTPContextReturnsOpenAIErrorForOversizedBody(t *testing.T) {
	host, contextID := newTestHost(t, modelConfig("secret"), aiRouteName("config-1"))
	headers := [][2]string{{":method", "POST"}, {":path", "/v1/chat/completions"}}

	if action := host.CallOnRequestHeaders(contextID, headers, false); action != types.ActionPause {
		t.Fatalf("OnHttpRequestHeaders(oversized body) action = %v, want pause", action)
	}
	body := make([]byte, aiproxyruntime.MaxRequestBodyBytes+1)
	if action := host.CallOnRequestBody(contextID, body, true); action != types.ActionPause {
		t.Fatalf("OnHttpRequestBody(oversized body) action = %v, want pause", action)
	}
	response := host.GetSentLocalResponse(contextID)
	if response == nil {
		t.Fatal("OnHttpRequestBody(oversized body) local response = nil, want response")
	}
	if response.StatusCode != 413 {
		t.Errorf("OnHttpRequestBody(oversized body) status = %d, want 413", response.StatusCode)
	}
	if body := string(response.Data); !strings.Contains(body, `"error"`) || !strings.Contains(body, `"request_too_large"`) {
		t.Errorf("OnHttpRequestBody(oversized body) body = %q, want OpenAI-compatible request_too_large error", body)
	}
}

func newTestHost(t *testing.T, cfg config.PluginConfig, routeName string) (proxytest.HostEmulator, uint32) {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal(test plugin config) error = %v", err)
	}
	option := proxytest.NewEmulatorOption().
		WithPluginContext(func(contextID uint32) types.PluginContext {
			return &pluginContext{runner: policy.NewRunner()}
		}).
		WithPluginConfiguration(data).
		WithProperty([]string{"xds", "route_name"}, []byte(routeName))
	host, reset := proxytest.NewHostEmulator(option)
	t.Cleanup(reset)
	if status := host.StartPlugin(); status != types.OnPluginStartStatusOK {
		t.Fatalf("StartPlugin() = %v, want OK", status)
	}
	return host, host.InitializeHttpContext()
}

func modelConfig(apiKey string) config.PluginConfig {
	return config.PluginConfig{Routes: []config.RouteConfig{
		{
			GatewayName: "gateway-1",
			RouteName:   "route-1",
			RuleName:    "chat",
			ConfigID:    "config-1",
			APIKey:      apiKey,
			Models: []config.ModelConfig{
				{
					Model:         "assistant",
					UpstreamModel: "gpt-4o-mini",
				},
			},
		},
	}}
}

func aiRouteName(configID string) string {
	return "ingate-route/gateway-1/route-1/chat/POST/ai/" + configID
}

func headerValues(headers [][2]string, name string) []string {
	var values []string
	for _, header := range headers {
		if strings.EqualFold(header[0], name) {
			values = append(values, header[1])
		}
	}
	return values
}

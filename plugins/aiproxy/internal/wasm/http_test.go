package wasm

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/lgc202/ingate/pkg/llm"
	"github.com/lgc202/ingate/pkg/llm/sse"
	config "github.com/lgc202/ingate/pkg/plugin/aiproxy"
	"github.com/lgc202/ingate/plugins/aiproxy/internal/policy"
	aiproxyruntime "github.com/lgc202/ingate/plugins/aiproxy/internal/runtime"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

func TestHTTPContextRewritesRequestsForModelProviders(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		wantCluster string
		wantPath    string
		wantHeaders map[string]string
		checkBody   func(*testing.T, []byte)
	}{
		{
			name:        "OpenAI-compatible",
			model:       "assistant",
			wantCluster: "model/openai",
			wantPath:    "/v1/chat/completions",
			wantHeaders: map[string]string{
				"authorization": "Bearer openai-secret",
			},
			checkBody: func(t *testing.T, body []byte) {
				t.Helper()
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(body, &fields); err != nil {
					t.Fatalf("json.Unmarshal(OpenAI request body) error = %v", err)
				}
				if got := string(fields["model"]); got != `"gpt-4o-mini"` {
					t.Errorf("OpenAI request model = %s, want %q", got, "gpt-4o-mini")
				}
			},
		},
		{
			name:        "Anthropic",
			model:       "claude",
			wantCluster: "model/anthropic",
			wantPath:    "/v1/messages",
			wantHeaders: map[string]string{
				"x-api-key":         "anthropic-secret",
				"anthropic-version": "2023-06-01",
			},
			checkBody: func(t *testing.T, body []byte) {
				t.Helper()
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(body, &fields); err != nil {
					t.Fatalf("json.Unmarshal(Anthropic request body) error = %v", err)
				}
				if got := string(fields["model"]); got != `"claude-sonnet-4"` {
					t.Errorf("Anthropic request model = %s, want %q", got, "claude-sonnet-4")
				}
				if got := string(fields["max_tokens"]); got != "4096" {
					t.Errorf("Anthropic request max_tokens = %s, want 4096", got)
				}
			},
		},
		{
			name:        "Gemini",
			model:       "gemini",
			wantCluster: "model/gemini",
			wantPath:    "/v1beta/models/gemini-2.5-flash:generateContent",
			wantHeaders: map[string]string{
				"x-goog-api-key": "gemini-secret",
			},
			checkBody: func(t *testing.T, body []byte) {
				t.Helper()
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(body, &fields); err != nil {
					t.Fatalf("json.Unmarshal(Gemini request body) error = %v", err)
				}
				if _, exists := fields["model"]; exists {
					t.Errorf("Gemini request body = %s, want model encoded in request path only", body)
				}
				if _, exists := fields["contents"]; !exists {
					t.Errorf("Gemini request body = %s, want contents", body)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, contextID := newTestHost(t, modelConfig(), aiRouteName("config-1"))
			headers := [][2]string{
				{":method", "POST"},
				{":path", "/v1/chat/completions"},
				{":authority", "gateway.example.com"},
				{"content-type", "application/json"},
				{"content-type", "text/plain"},
				{"content-length", "128"},
				{"content-encoding", "gzip"},
				{"accept-encoding", "gzip"},
				{"authorization", "Bearer client-token"},
				{"x-api-key", "client-anthropic-key"},
				{"anthropic-version", "client-version"},
				{"x-goog-api-key", "client-gemini-key"},
				{aiClusterHeader, "client-cluster"},
				{aiRouteHeader, "client-route-config"},
			}

			if action := host.CallOnRequestHeaders(contextID, headers, false); action != types.ActionPause {
				t.Fatalf("OnHttpRequestHeaders(%s request) action = %v, want pause", tt.name, action)
			}
			for _, name := range []string{
				"authorization",
				"x-api-key",
				"anthropic-version",
				"x-goog-api-key",
				aiClusterHeader,
				aiRouteHeader,
				"content-length",
				"content-encoding",
				"content-type",
				"accept-encoding",
			} {
				if values := headerValues(host.GetCurrentRequestHeaders(contextID), name); len(values) != 0 {
					t.Errorf("OnHttpRequestHeaders(%s request) header %q = %v, want removed", tt.name, name, values)
				}
			}

			body := []byte(`{"model":"` + tt.model + `","messages":[{"role":"user","content":"hello"}]}`)
			if action := host.CallOnRequestBody(contextID, body, true); action != types.ActionContinue {
				t.Fatalf("OnHttpRequestBody(%s request) action = %v, want continue", tt.name, action)
			}
			requestHeaders := host.GetCurrentRequestHeaders(contextID)
			assertSingleHeader(t, requestHeaders, ":path", tt.wantPath)
			assertSingleHeader(t, requestHeaders, ":authority", "gateway.example.com")
			assertSingleHeader(t, requestHeaders, aiClusterHeader, tt.wantCluster)
			assertSingleHeader(t, requestHeaders, aiRouteHeader, "config-1")
			assertSingleHeader(t, requestHeaders, "content-type", jsonContentType)
			assertSingleHeader(t, requestHeaders, "content-length", strconv.Itoa(len(host.GetCurrentRequestBody(contextID))))
			for name, value := range tt.wantHeaders {
				assertSingleHeader(t, requestHeaders, name, value)
			}
			for _, name := range []string{"authorization", "x-api-key", "anthropic-version", "x-goog-api-key"} {
				if _, expected := tt.wantHeaders[name]; expected {
					continue
				}
				if values := headerValues(requestHeaders, name); len(values) != 0 {
					t.Errorf("OnHttpRequestBody(%s request) header %q = %v, want none", tt.name, name, values)
				}
			}
			tt.checkBody(t, host.GetCurrentRequestBody(contextID))
		})
	}
}

func TestHTTPContextDoesNotForwardClientAuthorizationWithoutAPIKey(t *testing.T) {
	cfg := modelConfig()
	cfg.Routes[0].Targets[0].APIKey = ""
	cfg.Routes[0].Targets[0].APIKeyHeader = ""
	cfg.Routes[0].Targets[0].APIKeyPrefix = ""
	host, contextID := newTestHost(t, cfg, aiRouteName("config-1"))
	headers := [][2]string{
		{":method", "POST"},
		{":path", "/v1/chat/completions"},
		{"authorization", "Bearer client-token"},
	}

	if action := host.CallOnRequestHeaders(contextID, headers, false); action != types.ActionPause {
		t.Fatalf("OnHttpRequestHeaders(API key omitted) action = %v, want pause", action)
	}
	if action := host.CallOnRequestBody(contextID, chatRequest("assistant", false), true); action != types.ActionContinue {
		t.Fatalf("OnHttpRequestBody(API key omitted) action = %v, want continue", action)
	}
	if got := headerValues(host.GetCurrentRequestHeaders(contextID), "authorization"); len(got) != 0 {
		t.Errorf("OnHttpRequestBody(API key omitted) authorization headers = %v, want none", got)
	}
}

func TestHTTPContextTransformsBufferedProviderResponses(t *testing.T) {
	tests := []struct {
		name             string
		model            string
		body             string
		wantID           string
		wantContent      string
		wantPromptTokens int
		wantOutputTokens int
		wantTotalTokens  int
	}{
		{
			name:  "OpenAI-compatible",
			model: "assistant",
			body: `{
				"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"gpt-4o-mini",
				"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],
				"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}
			}`,
			wantID: "chatcmpl-1", wantContent: "hello",
			wantPromptTokens: 3, wantOutputTokens: 2, wantTotalTokens: 5,
		},
		{
			name:  "Anthropic",
			model: "claude",
			body: `{
				"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4",
				"content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}],
				"stop_reason":"end_turn","usage":{"input_tokens":7,"output_tokens":3}
			}`,
			wantID: "msg_1", wantContent: "hello world",
			wantPromptTokens: 7, wantOutputTokens: 3, wantTotalTokens: 10,
		},
		{
			name:  "Gemini",
			model: "gemini",
			body: `{
				"candidates":[{"content":{"parts":[{"text":"hello "},{"text":"world"}],"role":"model"},"finishReason":"STOP","index":0}],
				"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":2,"totalTokenCount":10},
				"modelVersion":"gemini-2.5-flash","responseId":"resp_1"
			}`,
			wantID: "resp_1", wantContent: "hello world",
			wantPromptTokens: 8, wantOutputTokens: 2, wantTotalTokens: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, contextID := startAIRequest(t, tt.model, false)
			responseHeaders := [][2]string{
				{":status", "200"},
				{"content-type", "application/octet-stream"},
				{"content-type", "text/plain"},
				{"content-length", "256"},
				{"content-encoding", "gzip"},
			}
			if action := host.CallOnResponseHeaders(contextID, responseHeaders, false); action != types.ActionPause {
				t.Fatalf("OnHttpResponseHeaders(%s response) action = %v, want pause", tt.name, action)
			}
			currentHeaders := host.GetCurrentResponseHeaders(contextID)
			assertSingleHeader(t, currentHeaders, "content-type", jsonContentType)
			for _, name := range []string{"content-length", "content-encoding"} {
				if values := headerValues(currentHeaders, name); len(values) != 0 {
					t.Errorf("OnHttpResponseHeaders(%s response) header %q = %v, want removed", tt.name, name, values)
				}
			}

			body := []byte(tt.body)
			middle := len(body) / 2
			if action := host.CallOnResponseBody(contextID, body[:middle], false); action != types.ActionPause {
				t.Fatalf("OnHttpResponseBody(%s response first chunk) action = %v, want pause", tt.name, action)
			}
			if action := host.CallOnResponseBody(contextID, body[middle:], true); action != types.ActionContinue {
				t.Fatalf("OnHttpResponseBody(%s response final chunk) action = %v, want continue", tt.name, action)
			}

			var completion llm.ChatCompletion
			if err := json.Unmarshal(host.GetCurrentResponseBody(contextID), &completion); err != nil {
				t.Fatalf("json.Unmarshal(%s transformed response) error = %v; body=%s", tt.name, err, host.GetCurrentResponseBody(contextID))
			}
			if completion.ID != tt.wantID || completion.Model != tt.model {
				t.Errorf("%s transformed response metadata = %#v, want ID %q and public model %q", tt.name, completion, tt.wantID, tt.model)
			}
			if len(completion.Choices) != 1 || completion.Choices[0].Message.Content != tt.wantContent {
				t.Errorf("%s transformed response choices = %#v, want content %q", tt.name, completion.Choices, tt.wantContent)
			}
			if completion.Usage == nil ||
				completion.Usage.PromptTokens != tt.wantPromptTokens ||
				completion.Usage.CompletionTokens != tt.wantOutputTokens ||
				completion.Usage.TotalTokens != tt.wantTotalTokens {
				t.Errorf("%s transformed response usage = %#v, want %d/%d/%d", tt.name, completion.Usage, tt.wantPromptTokens, tt.wantOutputTokens, tt.wantTotalTokens)
			}
		})
	}
}

func TestHTTPContextTransformsProviderErrors(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		body        string
		wantMessage string
		wantType    string
		wantCode    string
	}{
		{
			name: "OpenAI-compatible", model: "assistant",
			body:        `{"error":{"message":"openai bad model","type":"invalid_request_error","param":"model","code":"bad_model"}}`,
			wantMessage: "openai bad model", wantType: "invalid_request_error", wantCode: "bad_model",
		},
		{
			name: "Anthropic", model: "claude",
			body:        `{"type":"error","error":{"type":"invalid_request_error","message":"anthropic bad model"}}`,
			wantMessage: "anthropic bad model", wantType: "invalid_request_error",
		},
		{
			name: "Gemini", model: "gemini",
			body:        `{"error":{"code":400,"message":"gemini bad model","status":"INVALID_ARGUMENT"}}`,
			wantMessage: "gemini bad model", wantType: "invalid_request_error", wantCode: "INVALID_ARGUMENT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, contextID := startAIRequest(t, tt.model, true)
			responseHeaders := [][2]string{{":status", "400"}, {"content-type", "text/event-stream"}}
			if action := host.CallOnResponseHeaders(contextID, responseHeaders, false); action != types.ActionPause {
				t.Fatalf("OnHttpResponseHeaders(%s error) action = %v, want pause", tt.name, action)
			}
			assertSingleHeader(t, host.GetCurrentResponseHeaders(contextID), "content-type", jsonContentType)
			if action := host.CallOnResponseBody(contextID, []byte(tt.body), true); action != types.ActionContinue {
				t.Fatalf("OnHttpResponseBody(%s error) action = %v, want continue", tt.name, action)
			}

			var envelope llm.ErrorEnvelope
			if err := json.Unmarshal(host.GetCurrentResponseBody(contextID), &envelope); err != nil {
				t.Fatalf("json.Unmarshal(%s transformed error) error = %v; body=%s", tt.name, err, host.GetCurrentResponseBody(contextID))
			}
			if envelope.Error.Message != tt.wantMessage || envelope.Error.Type != tt.wantType || envelope.Error.Code != tt.wantCode {
				t.Errorf("%s transformed error = %#v, want message %q type %q code %q", tt.name, envelope.Error, tt.wantMessage, tt.wantType, tt.wantCode)
			}
		})
	}
}

func TestHTTPContextReturns502WhenBufferedResponseCannotBeConverted(t *testing.T) {
	host, contextID := startAIRequest(t, "claude", false)
	responseHeaders := [][2]string{{":status", "200"}, {"content-type", "application/json"}}
	if action := host.CallOnResponseHeaders(contextID, responseHeaders, false); action != types.ActionPause {
		t.Fatalf("OnHttpResponseHeaders(invalid provider response) action = %v, want pause", action)
	}
	if action := host.CallOnResponseBody(contextID, []byte(`{"unexpected":true}`), true); action != types.ActionPause {
		t.Fatalf("OnHttpResponseBody(invalid provider response) action = %v, want pause after replacement response", action)
	}
	assertServerErrorResponse(t, host.GetSentLocalResponse(contextID), "invalid provider response")
}

func TestHTTPContextRejectsOversizedResponseImmediately(t *testing.T) {
	host, contextID := startAIRequest(t, "claude", false)
	responseHeaders := [][2]string{{":status", "200"}, {"content-type", "application/json"}}
	if action := host.CallOnResponseHeaders(contextID, responseHeaders, false); action != types.ActionPause {
		t.Fatalf("OnHttpResponseHeaders(oversized response) action = %v, want pause", action)
	}
	body := make([]byte, aiproxyruntime.MaxResponseBodyBytes+1)
	if action := host.CallOnResponseBody(contextID, body, false); action != types.ActionPause {
		t.Fatalf("OnHttpResponseBody(oversized response before end) action = %v, want pause after replacement response", action)
	}
	assertServerErrorResponse(t, host.GetSentLocalResponse(contextID), "oversized response")
	if action := host.CallOnResponseBody(contextID, []byte("raw vendor tail"), true); action != types.ActionPause {
		t.Errorf("OnHttpResponseBody(oversized response tail) action = %v, want pause to keep vendor data blocked", action)
	}
}

func TestHTTPContextNormalizesResponseWithoutBody(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		wantStatus uint32
		wantType   string
		wantCode   string
	}{
		{name: "upstream error", status: "429", wantStatus: 429, wantType: "rate_limit_error", wantCode: "429"},
		{name: "empty success", status: "200", wantStatus: 502, wantType: "server_error", wantCode: "502"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, contextID := startAIRequest(t, "gemini", false)
			responseHeaders := [][2]string{{":status", tt.status}, {"content-type", "application/json"}}
			if action := host.CallOnResponseHeaders(contextID, responseHeaders, true); action != types.ActionPause {
				t.Fatalf("OnHttpResponseHeaders(%s without body) action = %v, want pause after replacement response", tt.name, action)
			}

			response := host.GetSentLocalResponse(contextID)
			if response == nil {
				t.Fatalf("OnHttpResponseHeaders(%s without body) local response = nil, want normalized response", tt.name)
			}
			if response.StatusCode != tt.wantStatus {
				t.Errorf("OnHttpResponseHeaders(%s without body) status = %d, want %d", tt.name, response.StatusCode, tt.wantStatus)
			}
			assertSingleHeader(t, response.Headers, contentTypeHeader, jsonContentType)
			var envelope llm.ErrorEnvelope
			if err := json.Unmarshal(response.Data, &envelope); err != nil {
				t.Fatalf("json.Unmarshal(OnHttpResponseHeaders(%s without body) response) error = %v; body=%s", tt.name, err, response.Data)
			}
			if envelope.Error.Type != tt.wantType || envelope.Error.Code != tt.wantCode {
				t.Errorf("OnHttpResponseHeaders(%s without body) error = %#v, want type %q code %q", tt.name, envelope.Error, tt.wantType, tt.wantCode)
			}
		})
	}
}

func TestHTTPContextFailsClosedWhenResponseStreamCannotBeCreated(t *testing.T) {
	runtime := aiproxyruntime.Compile(config.PluginConfig{}, policy.NewRunner())
	option := proxytest.NewEmulatorOption().WithHttpContext(func(contextID uint32) types.HttpContext {
		return &httpContext{
			plugin: &pluginContext{runtime: runtime},
			responsePlan: &aiproxyruntime.ResponsePlan{
				Protocol:    config.Protocol("invalid"),
				PublicModel: "public-model",
				Stream:      true,
			},
		}
	})
	host, reset := proxytest.NewHostEmulator(option)
	t.Cleanup(reset)
	contextID := host.InitializeHttpContext()
	responseHeaders := [][2]string{
		{":status", "200"},
		{"content-type", "application/json"},
		{"content-type", "text/plain"},
	}
	if action := host.CallOnResponseHeaders(contextID, responseHeaders, false); action != types.ActionPause {
		t.Fatalf("OnHttpResponseHeaders(invalid stream protocol) action = %v, want pause after replacement response", action)
	}
	assertServerErrorResponse(t, host.GetSentLocalResponse(contextID), "invalid stream protocol")
}

func TestHTTPContextTransformsAnthropicSSEAcrossChunks(t *testing.T) {
	input := []byte(
		"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n" +
			"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
			"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
			"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":3}}\n\n" +
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	events := transformedStreamEvents(t, "claude", input, 17)
	if len(events) != 5 {
		t.Fatalf("Anthropic transformed SSE event count = %d, want 5", len(events))
	}
	var first llm.ChatCompletionChunk
	if err := json.Unmarshal(events[0].Data, &first); err != nil {
		t.Fatalf("json.Unmarshal(Anthropic first SSE event) error = %v", err)
	}
	if first.ID != "msg_1" || first.Model != "claude" || len(first.Choices) != 1 || first.Choices[0].Delta.Role != llm.RoleAssistant {
		t.Errorf("Anthropic first SSE event = %#v, want public model and assistant role", first)
	}
	var textChunk llm.ChatCompletionChunk
	if err := json.Unmarshal(events[1].Data, &textChunk); err != nil {
		t.Fatalf("json.Unmarshal(Anthropic text SSE event) error = %v", err)
	}
	if len(textChunk.Choices) != 1 || textChunk.Choices[0].Delta.Content == nil || *textChunk.Choices[0].Delta.Content != "hello" {
		t.Errorf("Anthropic text SSE event = %#v, want hello", textChunk)
	}
	var usageChunk llm.ChatCompletionChunk
	if err := json.Unmarshal(events[3].Data, &usageChunk); err != nil {
		t.Fatalf("json.Unmarshal(Anthropic usage SSE event) error = %v", err)
	}
	if usageChunk.Usage == nil || usageChunk.Usage.PromptTokens != 10 || usageChunk.Usage.CompletionTokens != 3 || usageChunk.Usage.TotalTokens != 13 {
		t.Errorf("Anthropic usage SSE event = %#v, want 10/3/13", usageChunk.Usage)
	}
	if got := string(events[4].Data); got != "[DONE]" {
		t.Errorf("Anthropic final SSE event = %q, want [DONE]", got)
	}
}

func TestHTTPContextTransformsGeminiSSEAcrossChunks(t *testing.T) {
	input := []byte(
		"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hel\"}],\"role\":\"model\"},\"index\":0}],\"responseId\":\"resp_1\"}\n\n" +
			"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"lo\"}],\"role\":\"model\"},\"finishReason\":\"STOP\",\"index\":0}],\"usageMetadata\":{\"promptTokenCount\":5,\"candidatesTokenCount\":2,\"totalTokenCount\":7},\"responseId\":\"resp_1\"}\n\n")
	events := transformedStreamEvents(t, "gemini", input, 11)
	if len(events) != 3 {
		t.Fatalf("Gemini transformed SSE event count = %d, want 3", len(events))
	}
	var first llm.ChatCompletionChunk
	if err := json.Unmarshal(events[0].Data, &first); err != nil {
		t.Fatalf("json.Unmarshal(Gemini first SSE event) error = %v", err)
	}
	if first.ID != "resp_1" || first.Model != "gemini" || len(first.Choices) != 1 || first.Choices[0].Delta.Content == nil || *first.Choices[0].Delta.Content != "hel" {
		t.Errorf("Gemini first SSE event = %#v, want public model and text hel", first)
	}
	var second llm.ChatCompletionChunk
	if err := json.Unmarshal(events[1].Data, &second); err != nil {
		t.Fatalf("json.Unmarshal(Gemini second SSE event) error = %v", err)
	}
	if second.Usage == nil || second.Usage.PromptTokens != 5 || second.Usage.CompletionTokens != 2 || second.Usage.TotalTokens != 7 {
		t.Errorf("Gemini second SSE usage = %#v, want 5/2/7", second.Usage)
	}
	if got := string(events[2].Data); got != "[DONE]" {
		t.Errorf("Gemini final SSE event = %q, want [DONE]", got)
	}
}

func TestHTTPContextReplacesInvalidStreamAndDropsVendorTail(t *testing.T) {
	host, contextID := startAIRequest(t, "claude", true)
	responseHeaders := [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}
	if action := host.CallOnResponseHeaders(contextID, responseHeaders, false); action != types.ActionContinue {
		t.Fatalf("OnHttpResponseHeaders(invalid Anthropic stream) action = %v, want continue", action)
	}
	if action := host.CallOnResponseBody(contextID, []byte("data: not-json\n\n"), false); action != types.ActionContinue {
		t.Fatalf("OnHttpResponseBody(invalid Anthropic stream) action = %v, want continue with normalized SSE error", action)
	}
	events := decodeSSEEvents(t, host.GetCurrentResponseBody(contextID), "invalid Anthropic stream")
	if len(events) != 2 {
		t.Fatalf("OnHttpResponseBody(invalid Anthropic stream) event count = %d, want 2", len(events))
	}
	var envelope llm.ErrorEnvelope
	if err := json.Unmarshal(events[0].Data, &envelope); err != nil {
		t.Fatalf("json.Unmarshal(invalid Anthropic stream error) error = %v; data=%s", err, events[0].Data)
	}
	if envelope.Error.Type != "server_error" || envelope.Error.Code != "502" {
		t.Errorf("OnHttpResponseBody(invalid Anthropic stream) error = %#v, want server_error with code 502", envelope.Error)
	}
	if got := string(events[1].Data); got != "[DONE]" {
		t.Errorf("OnHttpResponseBody(invalid Anthropic stream) final event = %q, want [DONE]", got)
	}
	if action := host.CallOnResponseBody(contextID, []byte("data: raw vendor tail\n\n"), true); action != types.ActionContinue {
		t.Fatalf("OnHttpResponseBody(invalid Anthropic stream tail) action = %v, want continue after dropping tail", action)
	}
	if got := host.GetCurrentResponseBody(contextID); len(got) != 0 {
		t.Errorf("OnHttpResponseBody(invalid Anthropic stream tail) body = %q, want empty", got)
	}
}

func TestHTTPContextPreservesTrafficOutsideConfiguredAIRoute(t *testing.T) {
	host, contextID := newTestHost(t, modelConfig(), "ingate-route/gateway-1/route-1/ordinary/POST")
	headers := [][2]string{
		{":method", "POST"},
		{":path", "/ordinary"},
		{"authorization", "Bearer client-token"},
		{"content-length", "12"},
		{aiClusterHeader, "client-value"},
	}

	if action := host.CallOnRequestHeaders(contextID, headers, true); action != types.ActionContinue {
		t.Fatalf("OnHttpRequestHeaders(non-AI route) action = %v, want continue", action)
	}
	requestHeaders := host.GetCurrentRequestHeaders(contextID)
	assertSingleHeader(t, requestHeaders, "authorization", "Bearer client-token")
	assertSingleHeader(t, requestHeaders, "content-length", "12")
	assertSingleHeader(t, requestHeaders, aiClusterHeader, "client-value")

	responseHeaders := [][2]string{{":status", "200"}, {"content-type", "text/plain"}, {"content-length", "2"}}
	if action := host.CallOnResponseHeaders(contextID, responseHeaders, false); action != types.ActionContinue {
		t.Fatalf("OnHttpResponseHeaders(non-AI route) action = %v, want continue", action)
	}
	responseBody := []byte("ok")
	if action := host.CallOnResponseBody(contextID, responseBody, true); action != types.ActionContinue {
		t.Fatalf("OnHttpResponseBody(non-AI route) action = %v, want continue", action)
	}
	if got := string(host.GetCurrentResponseBody(contextID)); got != "ok" {
		t.Errorf("OnHttpResponseBody(non-AI route) body = %q, want %q", got, "ok")
	}
}

func TestHTTPContextFailsClosedWhenAIRouteConfigIsStale(t *testing.T) {
	host, contextID := newTestHost(t, modelConfig(), aiRouteName("config-2"))
	headers := [][2]string{
		{":method", "POST"},
		{":path", "/v1/chat/completions"},
		{"authorization", "Bearer client-token"},
		{"x-api-key", "client-anthropic-key"},
		{"x-goog-api-key", "client-gemini-key"},
		{aiClusterHeader, "client-cluster"},
		{aiRouteHeader, "client-route-config"},
	}

	if action := host.CallOnRequestHeaders(contextID, headers, false); action != types.ActionPause {
		t.Fatalf("OnHttpRequestHeaders(stale AI config) action = %v, want pause", action)
	}
	for _, name := range []string{"authorization", "x-api-key", "x-goog-api-key", aiClusterHeader, aiRouteHeader} {
		if values := headerValues(host.GetCurrentRequestHeaders(contextID), name); len(values) != 0 {
			t.Errorf("OnHttpRequestHeaders(stale AI config) header %q = %v, want removed", name, values)
		}
	}
	response := host.GetSentLocalResponse(contextID)
	if response == nil {
		t.Fatal("OnHttpRequestHeaders(stale AI config) local response = nil, want response")
	}
	if response.StatusCode != 500 {
		t.Errorf("OnHttpRequestHeaders(stale AI config) status = %d, want 500", response.StatusCode)
	}
	if body := string(response.Data); !strings.Contains(body, `"code":"internal_error"`) {
		t.Errorf("OnHttpRequestHeaders(stale AI config) body = %q, want internal_error", body)
	}
}

func TestHTTPContextReturnsOpenAIErrorForInvalidJSON(t *testing.T) {
	host, contextID := newTestHost(t, modelConfig(), aiRouteName("config-1"))
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
	if body := string(response.Data); !strings.Contains(body, `"error"`) || !strings.Contains(body, `"invalid_request"`) {
		t.Errorf("OnHttpRequestBody(invalid JSON) body = %q, want OpenAI-compatible invalid_request error", body)
	}
}

func TestHTTPContextRejectsUnsupportedFieldsForEveryProvider(t *testing.T) {
	for _, model := range []string{"assistant", "claude", "gemini"} {
		t.Run(model, func(t *testing.T) {
			host, contextID := newTestHost(t, modelConfig(), aiRouteName("config-1"))
			headers := [][2]string{{":method", "POST"}, {":path", "/v1/chat/completions"}}
			if action := host.CallOnRequestHeaders(contextID, headers, false); action != types.ActionPause {
				t.Fatalf("OnHttpRequestHeaders(model %q with unsupported field) action = %v, want pause", model, action)
			}
			body := []byte(`{"model":"` + model + `","messages":[{"role":"user","content":"hello"}],"frequency_penalty":0.2}`)
			if action := host.CallOnRequestBody(contextID, body, true); action != types.ActionPause {
				t.Fatalf("OnHttpRequestBody(model %q with unsupported field) action = %v, want pause", model, action)
			}
			response := host.GetSentLocalResponse(contextID)
			if response == nil {
				t.Fatalf("OnHttpRequestBody(model %q with unsupported field) local response = nil, want 400 response", model)
			}
			if response.StatusCode != 400 {
				t.Errorf("OnHttpRequestBody(model %q with unsupported field) status = %d, want 400", model, response.StatusCode)
			}
			if body := string(response.Data); !strings.Contains(body, `"code":"unsupported_feature"`) {
				t.Errorf("OnHttpRequestBody(model %q with unsupported field) body = %q, want unsupported_feature", model, body)
			}
		})
	}
}

func TestHTTPContextReturnsOpenAIErrorForOversizedBody(t *testing.T) {
	host, contextID := newTestHost(t, modelConfig(), aiRouteName("config-1"))
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

func startAIRequest(t *testing.T, model string, stream bool) (proxytest.HostEmulator, uint32) {
	t.Helper()
	host, contextID := newTestHost(t, modelConfig(), aiRouteName("config-1"))
	headers := [][2]string{{":method", "POST"}, {":path", "/v1/chat/completions"}, {"content-type", jsonContentType}}
	if action := host.CallOnRequestHeaders(contextID, headers, false); action != types.ActionPause {
		t.Fatalf("OnHttpRequestHeaders(model %q) action = %v, want pause", model, action)
	}
	if action := host.CallOnRequestBody(contextID, chatRequest(model, stream), true); action != types.ActionContinue {
		t.Fatalf("OnHttpRequestBody(model %q, stream %t) action = %v, want continue", model, stream, action)
	}
	return host, contextID
}

func transformedStreamEvents(t *testing.T, model string, input []byte, chunkSize int) []sse.Event {
	t.Helper()
	host, contextID := startAIRequest(t, model, true)
	responseHeaders := [][2]string{
		{":status", "200"},
		{"content-type", "application/json"},
		{"content-type", "text/plain"},
		{"content-length", "1024"},
		{"content-encoding", "gzip"},
	}
	if action := host.CallOnResponseHeaders(contextID, responseHeaders, false); action != types.ActionContinue {
		t.Fatalf("OnHttpResponseHeaders(model %q SSE) action = %v, want continue", model, action)
	}
	currentHeaders := host.GetCurrentResponseHeaders(contextID)
	assertSingleHeader(t, currentHeaders, "content-type", sseContentType)
	for _, name := range []string{"content-length", "content-encoding"} {
		if values := headerValues(currentHeaders, name); len(values) != 0 {
			t.Errorf("OnHttpResponseHeaders(model %q SSE) header %q = %v, want removed", model, name, values)
		}
	}

	var output []byte
	for start := 0; start < len(input); start += chunkSize {
		end := min(start+chunkSize, len(input))
		if action := host.CallOnResponseBody(contextID, input[start:end], false); action != types.ActionContinue {
			t.Fatalf("OnHttpResponseBody(model %q SSE input[%d:%d]) action = %v, want continue", model, start, end, action)
		}
		output = append(output, host.GetCurrentResponseBody(contextID)...)
	}
	if action := host.CallOnResponseBody(contextID, nil, true); action != types.ActionContinue {
		t.Fatalf("OnHttpResponseBody(model %q SSE end-of-stream) action = %v, want continue", model, action)
	}
	output = append(output, host.GetCurrentResponseBody(contextID)...)

	return decodeSSEEvents(t, output, "model "+model+" transformed output")
}

func modelConfig() config.PluginConfig {
	return config.PluginConfig{Routes: []config.RouteConfig{
		{
			GatewayName: "gateway-1",
			RouteName:   "route-1",
			RuleName:    "chat",
			ConfigID:    "config-1",
			Targets: []config.TargetConfig{
				{
					ID: "openai", Provider: "openai", Protocol: config.ProtocolOpenAI,
					Cluster: "model/openai", BasePath: "/v1",
					APIKey: "openai-secret", APIKeyHeader: "authorization", APIKeyPrefix: "Bearer ",
				},
				{
					ID: "anthropic", Provider: "anthropic", Protocol: config.ProtocolAnthropic,
					Cluster: "model/anthropic", BasePath: "/v1",
					APIKey: "anthropic-secret", APIKeyHeader: "x-api-key",
					Headers: []config.HeaderConfig{{Name: "anthropic-version", Value: "2023-06-01"}},
				},
				{
					ID: "gemini", Provider: "gemini", Protocol: config.ProtocolGemini,
					Cluster: "model/gemini", BasePath: "/v1beta",
					APIKey: "gemini-secret", APIKeyHeader: "x-goog-api-key",
				},
			},
			Models: []config.ModelConfig{
				{Model: "assistant", TargetID: "openai", UpstreamModel: "gpt-4o-mini"},
				{Model: "claude", TargetID: "anthropic", UpstreamModel: "claude-sonnet-4"},
				{Model: "gemini", TargetID: "gemini", UpstreamModel: "gemini-2.5-flash"},
			},
		},
	}}
}

func chatRequest(model string, stream bool) []byte {
	request := `{"model":"` + model + `","messages":[{"role":"user","content":"hello"}]`
	if stream {
		request += `,"stream":true`
	}
	return []byte(request + `}`)
}

func aiRouteName(configID string) string {
	return "ingate-route/gateway-1/route-1/chat/POST/ai/" + configID
}

func assertSingleHeader(t *testing.T, headers [][2]string, name, want string) {
	t.Helper()
	values := headerValues(headers, name)
	if len(values) != 1 || values[0] != want {
		t.Errorf("header %q = %v, want [%s]", name, values, want)
	}
}

func assertServerErrorResponse(t *testing.T, response *proxytest.LocalHttpResponse, source string) {
	t.Helper()
	if response == nil {
		t.Fatalf("%s local response = nil, want 502 response", source)
	}
	if response.StatusCode != 502 {
		t.Errorf("%s status = %d, want 502", source, response.StatusCode)
	}
	assertSingleHeader(t, response.Headers, contentTypeHeader, jsonContentType)
	var envelope llm.ErrorEnvelope
	if err := json.Unmarshal(response.Data, &envelope); err != nil {
		t.Fatalf("json.Unmarshal(%s error response) error = %v; body=%s", source, err, response.Data)
	}
	if envelope.Error.Type != "server_error" || envelope.Error.Code != "502" {
		t.Errorf("%s error = %#v, want server_error with code 502", source, envelope.Error)
	}
}

func decodeSSEEvents(t *testing.T, data []byte, source string) []sse.Event {
	t.Helper()
	var decoder sse.Decoder
	events, err := decoder.Push(data)
	if err != nil {
		t.Fatalf("sse.Decoder.Push(%s) error = %v; data=%s", source, err, data)
	}
	last, err := decoder.Finish()
	if err != nil {
		t.Fatalf("sse.Decoder.Finish(%s) error = %v; data=%s", source, err, data)
	}
	return append(events, last...)
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

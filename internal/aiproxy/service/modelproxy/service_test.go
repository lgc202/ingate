package modelproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"testing/synctest"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

type countedBody struct {
	reads  int
	closes int
}

func TestChatCompletionSendsPreparedRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := runTestService(t, fixtureConfigMap(t))
		body := []byte(`{"model":"coding","messages":[],"stream":true,"vendor":{"enabled":true}}`)
		original := bytes.Clone(body)
		params := ChatCompletionParams{
			ListenerPort: 8080, HostName: "gateway/team-a/public/listener/api",
			Body: body, Accept: []string{"text/event-stream", "application/json"},
		}

		response, err := service.ChatCompletion(t.Context(), params)
		if err != nil {
			t.Fatalf("ChatCompletion(coding) error = %v", err)
		}
		data := readResponse(t, response)

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatalf("Unmarshal(outbound body) error = %v", err)
		}
		wantFields := map[string]json.RawMessage{
			"model": json.RawMessage(`"code-v2"`), "messages": json.RawMessage(`[]`),
			"stream": json.RawMessage(`true`), "vendor": json.RawMessage(`{"enabled":true}`),
		}
		if diff := cmp.Diff(wantFields, fields); diff != "" {
			t.Errorf("ChatCompletion(coding) body mismatch (-want +got):\n%s", diff)
		}
		const target = "ai/target/96914ad662f712bc8376b8a6ec1aee8de3bb4f51dcd55f0670220c8fbe183d07"
		if response.Header.Get("X-Test-Target") != target || response.Header.Get("X-Test-Path") != "/v1/chat/completions" {
			t.Errorf("ChatCompletion(coding) target/path = %v, want original-model target and /v1 operation", response.Header)
		}
		if !bytes.Equal(body, original) {
			t.Errorf("ChatCompletion(coding) modified input to %s, want %s", body, original)
		}
	})
}

func TestChatCompletionWithoutModelOverride(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		configMap := fixtureConfigMap(t)
		compiled := decodeTestConfig(t, configMap)
		compiled.Backends[0].ModelNameOverride = new("")
		setTestConfig(t, configMap, *compiled)
		service, _ := runTestService(t, configMap)
		body := []byte("{\n  \"model\": \"coding\", \"vendor\": true\n}")
		original := bytes.Clone(body)

		response, err := service.ChatCompletion(t.Context(), modelParams(body))
		if err != nil {
			t.Fatalf("ChatCompletion(no override) error = %v", err)
		}
		data := readResponse(t, response)

		if !bytes.Equal(data, original) || !bytes.Equal(body, original) {
			t.Errorf("ChatCompletion(no override) outbound/source = %s/%s, want original bytes %s", data, body, original)
		}
	})
}

func TestChatCompletionUsesConfiguredEscapedPrefix(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		configMap := fixtureConfigMap(t)
		compiled := decodeTestConfig(t, configMap)
		compiled.Backends[0].Schema.Prefix = "/custom%20base/"
		setTestConfig(t, configMap, *compiled)
		service, _ := runTestService(t, configMap)

		response, err := service.ChatCompletion(t.Context(), modelParams([]byte(`{"model":"coding"}`)))
		if err != nil {
			t.Fatalf("ChatCompletion(escaped prefix) error = %v", err)
		}
		readResponse(t, response)

		if path := response.Header.Get("X-Test-Path"); path != "/custom%20base/chat/completions" {
			t.Errorf("ChatCompletion(escaped prefix) path = %q, want configured escaped prefix without duplicate version", path)
		}
	})
}

func TestChatCompletionRejectsInvalidRequests(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := runTestService(t, fixtureConfigMap(t))
		service.client.Transport = rejectUnexpectedRequest(t)
		for _, body := range []string{"", `null`, `{}`, `{"Model":"coding"}`, `{"model":""}`, `{"model":123}`, `{"model":"coding","model":"other"}`} {
			response, err := service.ChatCompletion(t.Context(), modelParams([]byte(body)))

			if response != nil || !errors.Is(err, ErrInvalidArgument) {
				t.Errorf("ChatCompletion(%q) = (%v, %v), want nil and ErrInvalidArgument", body, response, err)
			}
		}
		_, err := service.ChatCompletion(t.Context(), modelParams([]byte(`{"model":123}`)))

		if _, ok := errors.AsType[*json.UnmarshalTypeError](err); !ok {
			t.Errorf("ChatCompletion(numeric model) error = %v, want original JSON type error", err)
		}
	})
}

func TestChatCompletionDoesNotCrossEntries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := runTestService(t, fixtureConfigMap(t))
		service.client.Transport = rejectUnexpectedRequest(t)
		for _, tt := range []struct {
			name string
			port int32
			host string
			body string
		}{
			{name: "unknown model", port: 8080, host: "gateway/team-a/public/listener/api", body: `{"model":"other"}`},
			{name: "case-sensitive model", port: 8080, host: "gateway/team-a/public/listener/api", body: `{"model":"Coding"}`},
			{name: "overridden model", port: 8080, host: "gateway/team-a/public/listener/api", body: `{"model":"code-v2"}`},
			{name: "other namespace", port: 8080, host: "gateway/team-b/public/listener/api", body: `{"model":"coding"}`},
			{name: "other port", port: 8081, host: "gateway/team-a/public/listener/api", body: `{"model":"coding"}`},
			{name: "public hostname", port: 8080, host: "gateway.example.com", body: `{"model":"coding"}`},
		} {
			params := ChatCompletionParams{ListenerPort: tt.port, HostName: tt.host, Body: []byte(tt.body)}

			response, err := service.ChatCompletion(t.Context(), params)

			if response != nil || !errors.Is(err, ErrModelNotFound) || !errors.Is(err, aiconfig.ErrModelRouteNotFound) {
				t.Errorf("ChatCompletion(%s) = (%v, %v), want unmatched entry without outbound request", tt.name, response, err)
			}
		}
	})
}

func TestChatCompletionDoesNotFallThroughUnavailableBackend(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		configMap := fixtureConfigMap(t)
		compiled := decodeTestConfig(t, configMap)
		host := &compiled.Listeners[0].Hosts[0]
		fallback := host.ModelRoutes[0]
		fallback.Name = "team-a/coding/rule/1/match/0"
		fallback.Headers = []aidataplane.HeaderMatch{}
		host.ModelRoutes[0].BackendName = new("")
		host.ModelRoutes = append(host.ModelRoutes, fallback)
		setTestConfig(t, configMap, *compiled)
		service, _ := runTestService(t, configMap)
		service.client.Transport = rejectUnexpectedRequest(t)

		response, err := service.ChatCompletion(t.Context(), modelParams([]byte(`{"model":"coding"}`)))

		if response != nil || !errors.Is(err, ErrUnavailable) || !errors.Is(err, aiconfig.ErrModelBackendUnavailable) {
			t.Errorf("ChatCompletion(unavailable first match) = (%v, %v), want original backend error without fallback", response, err)
		}
	})
}

func TestChatCompletionConfigurationChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		configMap := fixtureConfigMap(t)
		service, apiClient := runTestService(t, configMap)
		params := modelParams([]byte(`{"model":"coding"}`))
		previous, err := service.ChatCompletion(t.Context(), params)
		if err != nil {
			t.Fatalf("ChatCompletion(initial configuration) error = %v", err)
		}
		previousBody := readResponse(t, previous)

		compiled := decodeTestConfig(t, configMap)
		compiled.Backends[0].ModelNameOverride = new("code-v3")
		compiled.Backends[0].Schema.Prefix = "v2/"
		setTestConfig(t, configMap, *compiled)
		if _, err := apiClient.CoreV1().ConfigMaps(configMap.Namespace).Update(t.Context(), configMap, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("Update(configuration) error = %v", err)
		}
		synctest.Wait()

		current, err := service.ChatCompletion(t.Context(), params)
		if err != nil {
			t.Fatalf("ChatCompletion(updated configuration) error = %v", err)
		}
		currentBody := readResponse(t, current)

		if string(previousBody) != `{"model":"code-v2"}` || string(currentBody) != `{"model":"code-v3"}` || current.Header.Get("X-Test-Path") != "/v2/chat/completions" {
			t.Errorf("ChatCompletion(config update) previous/current/path = %s/%s/%q, want independent code-v2/code-v3 and /v2", previousBody, currentBody, current.Header.Get("X-Test-Path"))
		}
		if previous.Header.Get("X-Test-Target") == current.Header.Get("X-Test-Target") {
			t.Error("ChatCompletion(config update) reused target for different protocol content")
		}

		if err := apiClient.CoreV1().ConfigMaps(configMap.Namespace).Delete(t.Context(), configMap.Name, metav1.DeleteOptions{}); err != nil {
			t.Fatalf("Delete(configuration) error = %v", err)
		}
		synctest.Wait()
		service.client.Transport = rejectUnexpectedRequest(t)

		response, err := service.ChatCompletion(t.Context(), params)

		if response != nil || !errors.Is(err, ErrUnavailable) {
			t.Errorf("ChatCompletion(deleted configuration) = (%v, %v), want unavailable without outbound request", response, err)
		}
	})
}

func TestChatCompletionBeforeConfigurationAndCancelled(t *testing.T) {
	loader, err := aiconfig.NewLoader(fake.NewClientset(), "control-plane", slog.Default())
	if err != nil {
		t.Fatalf("NewLoader(before load) error = %v", err)
	}
	service := New(loader, &http.Client{Transport: rejectUnexpectedRequest(t)})

	response, err := service.ChatCompletion(t.Context(), ChatCompletionParams{})

	if response != nil || !errors.Is(err, ErrUnavailable) {
		t.Errorf("ChatCompletion(before load) = (%v, %v), want unavailable", response, err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	response, err = service.ChatCompletion(ctx, ChatCompletionParams{})

	if response != nil || !errors.Is(err, context.Canceled) {
		t.Errorf("ChatCompletion(canceled) = (%v, %v), want canceled before parsing or networking", response, err)
	}
}

func TestChatCompletionPreservesClientFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := runTestService(t, fixtureConfigMap(t))
		cause := errors.New("private connection details")
		service.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, cause })

		response, err := service.ChatCompletion(t.Context(), modelParams([]byte(`{"model":"coding"}`)))

		if response != nil || !errors.Is(err, ErrForwardFailed) || !errors.Is(err, cause) {
			t.Errorf("ChatCompletion(client failure) = (%v, %v), want forwarding error with original cause", response, err)
		}
	})
}

func TestChatCompletionTransfersResponseBodyWithoutReading(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := runTestService(t, fixtureConfigMap(t))
		body := &countedBody{}
		upstream := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: body, ContentLength: -1}
		service.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return upstream, nil })

		response, err := service.ChatCompletion(t.Context(), modelParams([]byte(`{"model":"coding"}`)))

		if err != nil || response != upstream || body.reads != 0 || body.closes != 0 {
			t.Fatalf("ChatCompletion(stream response) = (%v, %v, reads %d, closes %d), want original response without consumption", response, err, body.reads, body.closes)
		}
		if err := response.Body.Close(); err != nil || body.closes != 1 {
			t.Errorf("Close(transferred response) = (%v, closes %d), want caller-owned single close", err, body.closes)
		}
	})
}

func TestChatCompletionRejectsUnexpectedProtocolUpgrade(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := runTestService(t, fixtureConfigMap(t))
		body := &countedBody{}
		service.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusSwitchingProtocols, Header: make(http.Header), Body: body}, nil
		})

		response, err := service.ChatCompletion(t.Context(), modelParams([]byte(`{"model":"coding"}`)))

		if response != nil || !errors.Is(err, ErrForwardFailed) || body.closes != 1 {
			t.Errorf("ChatCompletion(unrequested upgrade) = (%v, %v, closes %d), want rejection and connection cleanup", response, err, body.closes)
		}
	})
}

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	// Service 构造的 bytes.Reader 正文由原生 NopCloser 包装，Close 始终成功。
	defer func() { _ = request.Body.Close() }()

	return f(request)
}

func (b *countedBody) Read([]byte) (int, error) {
	b.reads++

	return 0, io.EOF
}

func (b *countedBody) Close() error {
	b.closes++

	return nil
}

// runTestService 在 synctest 中使用真实 Loader，仅将远端 HTTP 替换为原生 Transport 测试替身。
func runTestService(t *testing.T, configMap *corev1.ConfigMap) (*Service, *fake.Clientset) {
	t.Helper()
	apiClient := fake.NewClientset(configMap)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader, err := aiconfig.NewLoader(apiClient, configMap.Namespace, logger)
	if err != nil {
		t.Fatalf("NewLoader(test service) error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- loader.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("Run(stopped loader) error = %v, want context.Canceled", err)
		}
	})
	synctest.Wait()
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		if request.Method != http.MethodPost || request.URL.Host != "ingate-ai-egress" || request.URL.RawQuery != "" || request.GetBody != nil {
			t.Errorf("ChatCompletion() outbound request = %s %s, replay %t, want fixed POST without query or replay", request.Method, request.URL, request.GetBody != nil)
		}
		if request.ContentLength != int64(len(body)) || request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("ChatCompletion() outbound framing = %d/%v, want rewritten JSON length", request.ContentLength, request.Header)
		}
		return &http.Response{
			StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)),
			Header: http.Header{"X-Test-Target": {request.Header.Get(aidataplane.TargetHeader)}, "X-Test-Path": {request.URL.EscapedPath()}},
		}, nil
	})}

	return New(loader, client), apiClient
}

func rejectUnexpectedRequest(t *testing.T) http.RoundTripper {
	t.Helper()
	return roundTripFunc(func(request *http.Request) (*http.Response, error) {
		t.Errorf("unexpected outbound request %s %s; validation must reject before calling the Client", request.Method, request.URL)
		return nil, errors.New("unexpected outbound request")
	})
}

func modelParams(body []byte) ChatCompletionParams {
	return ChatCompletionParams{ListenerPort: 8080, HostName: "gateway/team-a/public/listener/api", Body: body}
}

func readResponse(t *testing.T, response *http.Response) []byte {
	t.Helper()
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("Close(test response) error = %v", err)
		}
	}()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(test response) error = %v", err)
	}

	return data
}

func fixtureConfigMap(t *testing.T) *corev1.ConfigMap {
	t.Helper()
	data, err := os.ReadFile("../../../pkg/aidataplane/testdata/v1.json")
	if err != nil {
		t.Fatalf("ReadFile(configuration fixture) error = %v", err)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "control-plane", Name: aidataplane.ConfigMapName, UID: "original", ResourceVersion: "1"},
		Data:       map[string]string{aidataplane.ConfigMapDataKey: string(data)},
	}
}

func decodeTestConfig(t *testing.T, configMap *corev1.ConfigMap) *aidataplane.Config {
	t.Helper()
	compiled, err := aidataplane.Decode([]byte(configMap.Data[aidataplane.ConfigMapDataKey]))
	if err != nil {
		t.Fatalf("Decode(test configuration) error = %v", err)
	}

	return compiled
}

func setTestConfig(t *testing.T, configMap *corev1.ConfigMap, compiled aidataplane.Config) {
	t.Helper()
	data, err := aidataplane.Encode(compiled)
	if err != nil {
		t.Fatalf("Encode(test configuration) error = %v", err)
	}
	configMap.Data[aidataplane.ConfigMapDataKey] = string(data)
}

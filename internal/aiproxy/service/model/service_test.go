package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"testing/synctest"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/apischema/openai"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestPrepareChatCompletion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := runTestService(t, fixtureConfigMap(t))
		body := []byte(`{"model":"coding","messages":[],"stream":true,"vendor":{"enabled":true}}`)
		original := bytes.Clone(body)
		params := ChatCompletionParams{
			ListenerPort: 8080,
			HostName:     "gateway/team-a/public/listener/api",
			Body:         body,
		}

		request, err := service.PrepareChatCompletion(t.Context(), params)
		if err != nil {
			t.Fatalf("PrepareChatCompletion(coding) error = %v, want success", err)
		}

		wantBackend := aiconfig.ModelBackend{
			RouteName:          "team-a/coding/rule/0/match/0",
			Name:               "team-a/model-service/route/coding/rule/0/ref/0",
			Schema:             aidataplane.APISchema{Name: aidataplane.APISchemaOpenAI, Prefix: "/v1"},
			ModelNameOverride:  "code-v2",
			NetworkBackendName: "team-a/model-network",
		}
		if diff := cmp.Diff(wantBackend, request.Backend); diff != "" {
			t.Errorf("PrepareChatCompletion(coding) backend mismatch (-want +got):\n%s", diff)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(request.Body, &fields); err != nil {
			t.Fatalf("PrepareChatCompletion(coding) body decoding error = %v", err)
		}
		wantFields := map[string]json.RawMessage{
			"model":    json.RawMessage(`"code-v2"`),
			"messages": json.RawMessage(`[]`),
			"stream":   json.RawMessage(`true`),
			"vendor":   json.RawMessage(`{"enabled":true}`),
		}
		if diff := cmp.Diff(wantFields, fields); diff != "" {
			t.Errorf("PrepareChatCompletion(coding) fields mismatch (-want +got):\n%s", diff)
		}
		if !bytes.Equal(body, original) {
			t.Errorf("PrepareChatCompletion(coding) changed source body to %s, want %s", body, original)
		}

		request.Body[0] = '!'
		request.Backend.ModelNameOverride = "changed"
		again, err := service.PrepareChatCompletion(t.Context(), params)
		if err != nil {
			t.Fatalf("PrepareChatCompletion(after result mutation) error = %v", err)
		}

		if !bytes.Equal(body, original) {
			t.Errorf("mutating request.Body changed source to %s, want %s", body, original)
		}
		var againFields map[string]json.RawMessage
		if err := json.Unmarshal(again.Body, &againFields); err != nil {
			t.Fatalf("PrepareChatCompletion(after result mutation) body decoding error = %v", err)
		}
		if diff := cmp.Diff(wantFields, againFields); diff != "" {
			t.Errorf("PrepareChatCompletion(after result mutation) fields mismatch (-want +got):\n%s", diff)
		}
		if again.Backend != wantBackend {
			t.Errorf("PrepareChatCompletion(after result mutation) backend = %+v, want %+v", again.Backend, wantBackend)
		}
	})
}

func TestPrepareChatCompletionWithoutModelOverride(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		configMap := fixtureConfigMap(t)
		compiled := decodeTestConfig(t, configMap)
		compiled.Backends[0].ModelNameOverride = new("")
		setTestConfig(t, configMap, *compiled)
		service, _ := runTestService(t, configMap)
		body := []byte("{\n  \"model\": \"coding\", \"vendor\": true\n}")
		original := bytes.Clone(body)

		request, err := service.PrepareChatCompletion(t.Context(), ChatCompletionParams{
			ListenerPort: 8080,
			HostName:     "gateway/team-a/public/listener/api",
			Body:         body,
		})
		if err != nil {
			t.Fatalf("PrepareChatCompletion(no override) error = %v", err)
		}

		if !bytes.Equal(request.Body, original) {
			t.Errorf("PrepareChatCompletion(no override) body = %s, want original bytes %s", request.Body, original)
		}

		body[0] = '!'

		if !bytes.Equal(request.Body, original) {
			t.Errorf("source mutation changed request.Body to %s, want independent bytes %s", request.Body, original)
		}
	})
}

func TestPrepareChatCompletionRejectsInvalidRequests(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := runTestService(t, fixtureConfigMap(t))
		for _, body := range []string{
			``,
			`null`,
			`{}`,
			`{"Model":"coding"}`,
			`{"model":""}`,
			`{"model":123}`,
			`{"model":"coding","model":"other"}`,
		} {
			params := ChatCompletionParams{
				ListenerPort: 8080,
				HostName:     "gateway/team-a/public/listener/api",
				Body:         []byte(body),
			}

			request, err := service.PrepareChatCompletion(t.Context(), params)

			if request != nil || !errors.Is(err, ErrInvalidArgument) {
				t.Errorf("PrepareChatCompletion(%q) = (%+v, %v), want nil and ErrInvalidArgument", body, request, err)
			}
		}

		_, err := service.PrepareChatCompletion(t.Context(), ChatCompletionParams{
			ListenerPort: 8080,
			HostName:     "gateway/team-a/public/listener/api",
			Body:         []byte(`{"model":123}`),
		})

		if _, ok := errors.AsType[*json.UnmarshalTypeError](err); !ok {
			t.Errorf("PrepareChatCompletion(numeric model) error = %v, want wrapped *json.UnmarshalTypeError", err)
		}
	})
}

func TestPrepareChatCompletionDoesNotCrossEntries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := runTestService(t, fixtureConfigMap(t))
		for _, tt := range []struct {
			name  string
			port  int32
			host  string
			model string
		}{
			{name: "unknown model", port: 8080, host: "gateway/team-a/public/listener/api", model: "other"},
			{name: "model case", port: 8080, host: "gateway/team-a/public/listener/api", model: "Coding"},
			{name: "overridden model", port: 8080, host: "gateway/team-a/public/listener/api", model: "code-v2"},
			{name: "other namespace", port: 8080, host: "gateway/team-b/public/listener/api", model: "coding"},
			{name: "other port", port: 8081, host: "gateway/team-a/public/listener/api", model: "coding"},
			{name: "public hostname", port: 8080, host: "gateway.example.com", model: "coding"},
		} {
			body, err := json.Marshal(map[string]string{"model": tt.model})
			if err != nil {
				t.Fatalf("Marshal(model %q) error = %v", tt.model, err)
			}
			params := ChatCompletionParams{ListenerPort: tt.port, HostName: tt.host, Body: body}

			request, err := service.PrepareChatCompletion(t.Context(), params)

			if request != nil || !errors.Is(err, ErrModelNotFound) {
				t.Errorf("PrepareChatCompletion(%s) = (%+v, %v), want nil and ErrModelNotFound", tt.name, request, err)
			}
			if !errors.Is(err, aiconfig.ErrModelRouteNotFound) {
				t.Errorf("PrepareChatCompletion(%s) error = %v, want preserved route error", tt.name, err)
			}
		}
	})
}

func TestPrepareChatCompletionRejectsUnavailableBackend(t *testing.T) {
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

		request, err := service.PrepareChatCompletion(t.Context(), ChatCompletionParams{
			ListenerPort: 8080,
			HostName:     "gateway/team-a/public/listener/api",
			Body:         []byte(`{"model":"coding"}`),
		})

		if request != nil || !errors.Is(err, ErrUnavailable) {
			t.Errorf("PrepareChatCompletion(unavailable first match) = (%+v, %v), want nil and ErrUnavailable", request, err)
		}
		if !errors.Is(err, aiconfig.ErrModelBackendUnavailable) {
			t.Errorf("PrepareChatCompletion(unavailable first match) error = %v, want preserved backend error", err)
		}
	})
}

func TestPrepareChatCompletionConfigurationChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		configMap := fixtureConfigMap(t)
		service, apiClient := runTestService(t, configMap)
		params := ChatCompletionParams{
			ListenerPort: 8080,
			HostName:     "gateway/team-a/public/listener/api",
			Body:         []byte(`{"model":"coding"}`),
		}
		previous, err := service.PrepareChatCompletion(t.Context(), params)
		if err != nil {
			t.Fatalf("PrepareChatCompletion(initial configuration) error = %v", err)
		}

		previousBody := bytes.Clone(previous.Body)
		compiled := decodeTestConfig(t, configMap)
		compiled.Backends[0].ModelNameOverride = new("code-v3")
		compiled.Backends[0].Schema.Prefix = "v2/"
		setTestConfig(t, configMap, *compiled)
		if _, err := apiClient.CoreV1().ConfigMaps(configMap.Namespace).Update(t.Context(), configMap, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("Update(configuration) error = %v", err)
		}
		synctest.Wait()

		current, err := service.PrepareChatCompletion(t.Context(), params)
		if err != nil {
			t.Fatalf("PrepareChatCompletion(updated configuration) error = %v", err)
		}

		if current.Backend.ModelNameOverride != "code-v3" || current.Backend.Schema.Prefix != "v2/" {
			t.Errorf("PrepareChatCompletion(updated configuration) backend = %+v, want code-v3 and v2/", current.Backend)
		}
		var currentBody openai.ChatCompletionRequest
		if err := json.Unmarshal(current.Body, &currentBody); err != nil {
			t.Fatalf("PrepareChatCompletion(updated configuration) body decoding error = %v", err)
		}
		if currentBody.Model != "code-v3" {
			t.Errorf("PrepareChatCompletion(updated configuration) model = %q, want code-v3", currentBody.Model)
		}
		if !bytes.Equal(previous.Body, previousBody) || previous.Backend.ModelNameOverride != "code-v2" || previous.Backend.Schema.Prefix != "/v1" {
			t.Errorf("configuration update changed previous request to %+v, want original result", previous)
		}

		if err := apiClient.CoreV1().ConfigMaps(configMap.Namespace).Delete(t.Context(), configMap.Name, metav1.DeleteOptions{}); err != nil {
			t.Fatalf("Delete(configuration) error = %v", err)
		}
		synctest.Wait()

		request, err := service.PrepareChatCompletion(t.Context(), params)

		if request != nil || !errors.Is(err, ErrUnavailable) {
			t.Errorf("PrepareChatCompletion(deleted configuration) = (%+v, %v), want nil and ErrUnavailable", request, err)
		}
	})
}

func TestPrepareChatCompletionBeforeConfigurationLoad(t *testing.T) {
	loader, err := aiconfig.NewLoader(fake.NewClientset(), "control-plane", slog.Default())
	if err != nil {
		t.Fatalf("NewLoader(before load) error = %v", err)
	}
	service := New(loader)

	request, err := service.PrepareChatCompletion(t.Context(), ChatCompletionParams{})

	if request != nil || !errors.Is(err, ErrUnavailable) {
		t.Errorf("PrepareChatCompletion(before load) = (%+v, %v), want nil and ErrUnavailable", request, err)
	}
}

func TestPrepareChatCompletionCancelled(t *testing.T) {
	loader, err := aiconfig.NewLoader(fake.NewClientset(), "control-plane", slog.Default())
	if err != nil {
		t.Fatalf("NewLoader(cancelled request) error = %v", err)
	}
	service := New(loader)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	request, err := service.PrepareChatCompletion(ctx, ChatCompletionParams{})

	if request != nil || !errors.Is(err, context.Canceled) {
		t.Errorf("PrepareChatCompletion(cancelled request) = (%+v, %v), want nil and context.Canceled", request, err)
	}
}

// runTestService 在 synctest 中启动真实 Loader，使用原生 fake Clientset 提供配置事件。
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
			t.Errorf("Run(stopped test loader) error = %v, want context.Canceled", err)
		}
	})
	synctest.Wait()
	if _, loaded := loader.Capture(); !loaded {
		t.Fatal("Capture(test service) has no configuration, want valid initial configuration")
	}

	return New(loader), apiClient
}

func fixtureConfigMap(t *testing.T) *corev1.ConfigMap {
	t.Helper()
	data, err := os.ReadFile("../../../pkg/aidataplane/testdata/v1.json")
	if err != nil {
		t.Fatalf("ReadFile(configuration fixture) error = %v", err)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "control-plane",
			Name:            aidataplane.ConfigMapName,
			UID:             "original",
			ResourceVersion: "1",
		},
		Data: map[string]string{aidataplane.ConfigMapDataKey: string(data)},
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

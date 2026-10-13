//go:build integration

package apiserver

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage/etcd3/testserver"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// TestResourceHTTPContract 通过真实 HTTPS 和隔离 etcd 验证已实现资源的存储契约。
func TestResourceHTTPContract(t *testing.T) {
	tests := []struct {
		name, kind, resource string
		spec, updatedSpec    map[string]any
	}{
		{
			name: "tools", kind: "MCPRoute", resource: "mcproutes",
			spec: map[string]any{
				"displayName": "重复展示名称",
				"parentRefs":  []any{map[string]any{"name": "missing-gateway"}},
				"path":        "/tools/mcp",
				"backendRefs": []any{map[string]any{"name": "missing-backend", "path": "/server/mcp"}},
			},
			updatedSpec: map[string]any{
				"displayName": "更新展示名称",
				"parentRefs":  []any{map[string]any{"name": "missing-gateway"}},
				"path":        "/tools/mcp/",
				"backendRefs": []any{map[string]any{"name": "another-missing-backend", "path": "/server/mcp/"}},
			},
		},
		{
			name: "models", kind: "ModelRoute", resource: "modelroutes",
			spec: map[string]any{
				"displayName": "重复展示名称",
				"parentRefs":  []any{map[string]any{"name": "missing-gateway"}},
				"rules": []any{map[string]any{
					"matches": []any{map[string]any{
						"headers": []any{map[string]any{"name": "x-ai-eg-model", "type": "Exact", "value": "coding"}},
					}},
					"backendRefs": []any{map[string]any{"name": "missing-ai-backend", "modelNameOverride": "code-v2"}},
				}},
			},
			updatedSpec: map[string]any{
				"displayName": "更新展示名称",
				"parentRefs":  []any{map[string]any{"name": "missing-gateway"}},
				"rules": []any{map[string]any{
					"backendRefs": []any{map[string]any{"name": "another-missing-ai-backend"}},
				}},
			},
		},
		{
			name: "model-service", kind: "AIBackend", resource: "aibackends",
			spec: map[string]any{
				"displayName": "重复展示名称",
				"backendRef":  map[string]any{"name": "missing-network"},
				"schema":      map[string]any{"name": "OpenAI", "prefix": "/v1"},
			},
			updatedSpec: map[string]any{
				"displayName": "更新展示名称",
				"backendRef":  map[string]any{"name": "another-missing-network"},
				"schema":      map[string]any{"name": "OpenAI", "prefix": "compatibility/v1"},
			},
		},
		{
			name: "edge", kind: "Gateway", resource: "gateways",
			spec: map[string]any{"displayName": "重复展示名称", "listeners": []any{
				map[string]any{"name": "http", "protocol": "HTTP", "port": int64(8080)},
			}},
			updatedSpec: map[string]any{"displayName": "更新展示名称", "listeners": []any{
				map[string]any{"name": "http", "protocol": "HTTP", "port": int64(8081)},
			}},
		},
		{
			name: "api", kind: "HTTPRoute", resource: "httproutes",
			spec: map[string]any{
				"displayName": "重复展示名称",
				"parentRefs":  []any{map[string]any{"name": "edge", "sectionName": "http"}},
				"rules": []any{map[string]any{
					"matches": []any{map[string]any{
						"path": map[string]any{"type": "PathPrefix", "value": "/api"},
					}},
					"backendRefs": []any{map[string]any{"name": "backend"}},
				}},
			},
			updatedSpec: map[string]any{
				"displayName": "更新展示名称",
				"parentRefs":  []any{map[string]any{"name": "edge", "sectionName": "http"}},
				"rules": []any{map[string]any{
					"matches": []any{map[string]any{
						"path": map[string]any{"type": "PathPrefix", "value": "/v1"},
					}},
					"backendRefs": []any{map[string]any{"name": "backend"}},
				}},
			},
		},
		{
			name: "backend", kind: "Backend", resource: "backends",
			spec: map[string]any{"displayName": "重复展示名称", "endpoints": []any{
				map[string]any{"address": "127.0.0.1", "port": int64(8080), "weight": int64(1)},
			}},
			updatedSpec: map[string]any{"displayName": "更新展示名称", "endpoints": []any{
				map[string]any{"address": "127.0.0.1", "port": int64(8081), "weight": int64(1)},
			}},
		},
	}

	client := startGatewayAPI(t)
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			resource := gatewayv1alpha1.SchemeGroupVersion.WithResource(tt.resource)
			testResourceHTTPContract(t, client.Resource(resource), tt.kind, tt.name, tt.spec, tt.updatedSpec)
		})
	}
}

func TestMCPRouteDefaultingAndValidation(t *testing.T) {
	client := startGatewayAPI(t)
	resources := client.Resource(gatewayv1alpha1.SchemeGroupVersion.WithResource("mcproutes"))
	route := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gatewayv1alpha1.SchemeGroupVersion.String(),
		"kind":       "MCPRoute",
		"metadata":   map[string]any{"name": "tools"},
		"spec": map[string]any{
			"parentRefs":  []any{map[string]any{"name": "missing-gateway"}},
			"backendRefs": []any{map[string]any{"name": "missing-backend"}},
		},
	}}
	created, err := resources.Create(t.Context(), route, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create(MCPRoute with unresolved references) error = %v, want nil", err)
	}
	var got gatewayv1alpha1.MCPRoute
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(created.Object, &got); err != nil {
		t.Fatalf("decode created MCPRoute error = %v, want nil", err)
	}
	if got.Spec.Path == nil || *got.Spec.Path != "/mcp" {
		t.Errorf("Create(MCPRoute).path = %v, want /mcp", got.Spec.Path)
	}
	if len(got.Spec.BackendRefs) != 1 {
		t.Fatalf("Create(MCPRoute).backendRefs = %+v, want one Backend", got.Spec.BackendRefs)
	}
	if got.Spec.BackendRefs[0].Path == nil || *got.Spec.BackendRefs[0].Path != "/mcp" {
		t.Errorf("Create(MCPRoute).backendRefs[0].path = %v, want /mcp", got.Spec.BackendRefs[0].Path)
	}

	for _, tt := range []struct {
		name string
		spec map[string]any
	}{
		{
			name: "empty-entry-path",
			spec: map[string]any{
				"parentRefs":  []any{map[string]any{"name": "missing-gateway"}},
				"path":        "",
				"backendRefs": []any{map[string]any{"name": "missing-backend"}},
			},
		},
		{
			name: "empty-upstream-path",
			spec: map[string]any{
				"parentRefs":  []any{map[string]any{"name": "missing-gateway"}},
				"backendRefs": []any{map[string]any{"name": "missing-backend", "path": ""}},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			invalid := route.DeepCopy()
			invalid.SetName(tt.name)
			invalid.Object["spec"] = tt.spec
			if _, err := resources.Create(t.Context(), invalid, metav1.CreateOptions{}); !apierrors.IsInvalid(err) {
				t.Errorf("Create(MCPRoute %s) error = %v, want Invalid", tt.name, err)
			}
		})
	}
	unknown := route.DeepCopy()
	unknown.SetName("unsupported-security-policy")
	unknown.Object["spec"].(map[string]any)["backendRefs"] = []any{map[string]any{
		"name": "missing-backend", "securityPolicy": map[string]any{},
	}}
	if _, err := resources.Create(t.Context(), unknown, metav1.CreateOptions{FieldValidation: metav1.FieldValidationStrict}); !apierrors.IsBadRequest(err) {
		t.Errorf("strict Create(MCPRoute securityPolicy) error = %v, want BadRequest", err)
	}
}

func TestHTTPRouteDefaultingAndValidation(t *testing.T) {
	client := startGatewayAPI(t)
	resources := client.Resource(gatewayv1alpha1.SchemeGroupVersion.WithResource("httproutes"))
	route := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gatewayv1alpha1.SchemeGroupVersion.String(), "kind": "HTTPRoute",
		"metadata": map[string]any{"name": "defaulted"},
		"spec": map[string]any{
			"parentRefs": []any{map[string]any{"name": "edge"}},
			"rules":      []any{map[string]any{"backendRefs": []any{map[string]any{"name": "backend"}}}},
		},
	}}
	created, err := resources.Create(t.Context(), route, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create(defaulted): %v", err)
	}
	rules, found, err := unstructured.NestedSlice(created.Object, "spec", "rules")
	if err != nil || !found || len(rules) != 1 {
		t.Fatalf("rules = %v, found %v, err %v", rules, found, err)
	}
	matches := rules[0].(map[string]any)["matches"].([]any)
	path := matches[0].(map[string]any)["path"].(map[string]any)
	if path["type"] != "PathPrefix" || path["value"] != "/" {
		t.Errorf("default path = %v, want PathPrefix /", path)
	}
	invalid := route.DeepCopy()
	invalid.SetName("invalid")
	invalid.Object["spec"].(map[string]any)["parentRefs"] = []any{map[string]any{"name": "edge"}, map[string]any{"name": "edge", "sectionName": "http"}}
	if _, err := resources.Create(t.Context(), invalid, metav1.CreateOptions{}); !apierrors.IsInvalid(err) {
		t.Errorf("overlapping parents error = %v, want Invalid", err)
	}
	unknown := route.DeepCopy()
	unknown.SetName("unsupported")
	unknown.Object["spec"].(map[string]any)["rules"] = []any{map[string]any{
		"matches":     []any{map[string]any{"method": "POST"}},
		"backendRefs": []any{map[string]any{"name": "backend"}},
	}}
	if _, err := resources.Create(t.Context(), unknown, metav1.CreateOptions{FieldValidation: metav1.FieldValidationStrict}); !apierrors.IsBadRequest(err) {
		t.Errorf("strict Create(unsupported method) error = %v, want BadRequest", err)
	}
}

func TestAIBackendDefaultingAndValidation(t *testing.T) {
	client := startGatewayAPI(t)
	resources := client.Resource(gatewayv1alpha1.SchemeGroupVersion.WithResource("aibackends"))
	backend := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gatewayv1alpha1.SchemeGroupVersion.String(),
		"kind":       "AIBackend",
		"metadata":   map[string]any{"name": "model-service"},
		"spec": map[string]any{
			"backendRef": map[string]any{"name": "missing-network"},
			"schema":     map[string]any{"name": "OpenAI"},
		},
	}}
	created, err := resources.Create(t.Context(), backend, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create(AIBackend with unresolved reference) error = %v, want nil", err)
	}
	prefix, found, err := unstructured.NestedString(created.Object, "spec", "schema", "prefix")
	if err != nil || !found || prefix != "/v1" {
		t.Errorf("Create(AIBackend).schema.prefix = %q, found %t, error %v, want /v1", prefix, found, err)
	}

	invalid := backend.DeepCopy()
	invalid.SetName("unsupported")
	invalid.Object["spec"].(map[string]any)["schema"] = map[string]any{"name": "Anthropic"}
	if _, err := resources.Create(t.Context(), invalid, metav1.CreateOptions{}); !apierrors.IsInvalid(err) {
		t.Errorf("Create(AIBackend unsupported schema) error = %v, want Invalid", err)
	}
	invalid.SetName("invalid-prefix")
	invalid.Object["spec"].(map[string]any)["schema"] = map[string]any{"name": "OpenAI", "prefix": "https://api.example.com/v1"}
	if _, err := resources.Create(t.Context(), invalid, metav1.CreateOptions{}); !apierrors.IsInvalid(err) {
		t.Errorf("Create(AIBackend URL prefix) error = %v, want Invalid", err)
	}
	invalid.SetName("unsupported-version")
	invalid.Object["spec"].(map[string]any)["schema"] = map[string]any{"name": "OpenAI", "version": "v1"}
	if _, err := resources.Create(t.Context(), invalid, metav1.CreateOptions{FieldValidation: metav1.FieldValidationStrict}); !apierrors.IsBadRequest(err) {
		t.Errorf("strict Create(AIBackend version) error = %v, want BadRequest", err)
	}
}

func TestModelRouteDefaultingAndValidation(t *testing.T) {
	client := startGatewayAPI(t)
	resources := client.Resource(gatewayv1alpha1.SchemeGroupVersion.WithResource("modelroutes"))
	route := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gatewayv1alpha1.SchemeGroupVersion.String(),
		"kind":       "ModelRoute",
		"metadata":   map[string]any{"name": "models"},
		"spec": map[string]any{
			"parentRefs": []any{map[string]any{"name": "missing-gateway"}},
			"rules": []any{map[string]any{
				"matches": []any{map[string]any{
					"headers": []any{map[string]any{"name": "X-AI-EG-MODEL", "value": "Coding"}},
				}},
				"backendRefs": []any{map[string]any{"name": "missing-ai-backend"}},
			}},
		},
	}}
	created, err := resources.Create(t.Context(), route, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create(ModelRoute with unresolved references) error = %v, want nil", err)
	}
	var got gatewayv1alpha1.ModelRoute
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(created.Object, &got); err != nil {
		t.Fatalf("decode created ModelRoute error = %v, want nil", err)
	}
	if len(got.Spec.Rules) != 1 || len(got.Spec.Rules[0].Matches) != 1 {
		t.Fatalf("Create(ModelRoute).rules = %+v, want one rule with one match", got.Spec.Rules)
	}
	wantHeader := gatewayv1alpha1.HTTPHeaderMatch{
		Name:  "X-AI-EG-MODEL",
		Type:  gatewayv1alpha1.HeaderMatchExact,
		Value: "Coding",
	}
	if diff := cmp.Diff([]gatewayv1alpha1.HTTPHeaderMatch{wantHeader}, got.Spec.Rules[0].Matches[0].Headers); diff != "" {
		t.Errorf("Create(ModelRoute).headers mismatch (-want +got):\n%s", diff)
	}

	invalid := route.DeepCopy()
	invalid.SetName("unsupported-match")
	invalidRules := invalid.Object["spec"].(map[string]any)["rules"].([]any)
	invalidMatches := invalidRules[0].(map[string]any)["matches"].([]any)
	invalidHeaders := invalidMatches[0].(map[string]any)["headers"].([]any)
	invalidHeaders[0].(map[string]any)["type"] = "RegularExpression"
	if _, err := resources.Create(t.Context(), invalid, metav1.CreateOptions{}); !apierrors.IsInvalid(err) {
		t.Errorf("Create(ModelRoute regex match) error = %v, want Invalid", err)
	}

	invalid = route.DeepCopy()
	invalid.SetName("unsupported-path")
	invalid.Object["spec"].(map[string]any)["path"] = "/v1/chat/completions"
	options := metav1.CreateOptions{FieldValidation: metav1.FieldValidationStrict}
	if _, err := resources.Create(t.Context(), invalid, options); !apierrors.IsBadRequest(err) {
		t.Errorf("strict Create(ModelRoute route-level path) error = %v, want BadRequest", err)
	}
}

func testResourceHTTPContract(
	t *testing.T,
	resources dynamic.ResourceInterface,
	kind, name string,
	spec, updatedSpec map[string]any,
) {
	t.Helper()
	ctx := t.Context()

	created, err := resources.Create(ctx, &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gatewayv1alpha1.SchemeGroupVersion.String(),
		"kind":       kind,
		"metadata": map[string]any{
			"name": name, "labels": map[string]any{"environment": "test"},
		},
		"spec":   spec,
		"status": statusForKind(kind, []any{map[string]any{"type": "Accepted", "status": "True"}}),
	}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create(%s) error = %v", kind, err)
	}
	if created.GetGeneration() != 1 || created.GetResourceVersion() == "" {
		t.Fatalf("Create(%s) metadata = %#v, want generation 1 and resourceVersion", kind, created.Object["metadata"])
	}
	assertConditions(t, created, 0)
	duplicate := created.DeepCopy()
	duplicate.SetName(name + "-duplicate")
	duplicate.SetResourceVersion("")
	duplicate.SetUID("")
	duplicate, err = resources.Create(ctx, duplicate, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create(%s, duplicate display name) error = %v, want nil", kind, err)
	}
	if duplicate.GetUID() == created.GetUID() || duplicate.GetUID() == "" {
		t.Errorf("Create(%s, duplicate display name) UID = %q, want distinct nonempty UID", kind, duplicate.GetUID())
	}
	if err := resources.Delete(ctx, duplicate.GetName(), metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Delete(%s duplicate) error = %v, want nil", kind, err)
	}

	got, err := resources.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(%s, %q) error = %v", kind, name, err)
	}
	if got.GetResourceVersion() != created.GetResourceVersion() {
		t.Errorf("Get(%s, %q) resourceVersion = %q, want %q",
			kind, name, got.GetResourceVersion(), created.GetResourceVersion())
	}

	listed, err := resources.List(ctx, metav1.ListOptions{
		LabelSelector: "environment=test",
		FieldSelector: "metadata.name=" + name,
	})
	if err != nil {
		t.Fatalf("List(%s) error = %v", kind, err)
	}
	if len(listed.Items) != 1 || listed.Items[0].GetName() != name {
		t.Fatalf("List(%s) items = %#v, want only %q", kind, listed.Items, name)
	}

	for _, selector := range []metav1.ListOptions{
		{LabelSelector: "environment=other"},
		{FieldSelector: "metadata.name=other"},
	} {
		listed, err := resources.List(ctx, selector)
		if err != nil {
			t.Fatalf("List(%s, %+v) error = %v", kind, selector, err)
		}
		if len(listed.Items) != 0 {
			t.Errorf("List(%s, %+v) items = %#v, want none", kind, selector, listed.Items)
		}
	}

	stream, err := resources.Watch(ctx, metav1.ListOptions{
		ResourceVersion: created.GetResourceVersion(), FieldSelector: "metadata.name=" + name,
	})
	if err != nil {
		t.Fatalf("Watch(%s) error = %v", kind, err)
	}
	defer stream.Stop()

	updated := created.DeepCopy()
	updated.Object["spec"] = updatedSpec
	updated.Object["status"] = statusForKind(kind, []any{map[string]any{"type": "Accepted", "status": "True"}})
	updated, err = resources.Update(ctx, updated, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Update(%s spec) error = %v", kind, err)
	}
	if updated.GetGeneration() != 2 || updated.GetResourceVersion() == created.GetResourceVersion() {
		t.Errorf("Update(%s spec) metadata = %#v, want new resourceVersion and generation 2",
			kind, updated.Object["metadata"])
	}
	assertSpec(t, updated, updatedSpec)
	if updated.GetName() != created.GetName() || updated.GetUID() != created.GetUID() {
		t.Errorf("Update(%s rename) name/UID = %q/%q, want %q/%q", kind, updated.GetName(), updated.GetUID(), created.GetName(), created.GetUID())
	}
	assertConditions(t, updated, 0)
	expectResourceEvent(t, stream, watch.Modified, name, updated.GetResourceVersion())

	if _, err := resources.Update(ctx, created.DeepCopy(), metav1.UpdateOptions{}); !apierrors.IsConflict(err) {
		t.Errorf("Update(%s, stale resourceVersion) error = %v, want Conflict", kind, err)
	}

	status := updated.DeepCopy()
	status.Object["spec"] = spec
	status.Object["status"] = statusForKind(kind, []any{
		map[string]any{
			"type": "Accepted", "status": "True", "reason": "Configured",
			"message": "ready", "observedGeneration": int64(2),
			"lastTransitionTime": metav1.Now().Format(time.RFC3339),
		},
	})
	status, err = resources.UpdateStatus(ctx, status, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("UpdateStatus(%s) error = %v", kind, err)
	}
	if status.GetGeneration() != 2 {
		t.Errorf("UpdateStatus(%s) generation = %d, want 2", kind, status.GetGeneration())
	}
	assertSpec(t, status, updatedSpec)
	assertConditions(t, status, 1)
	expectResourceEvent(t, stream, watch.Modified, name, status.GetResourceVersion())

	if err := resources.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Delete(%s, %q) error = %v", kind, name, err)
	}
	expectResourceEvent(t, stream, watch.Deleted, name, "")
	if _, err := resources.Get(ctx, name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Errorf("Get(deleted %s, %q) error = %v, want NotFound", kind, name, err)
	}
}

func startGatewayAPI(t *testing.T) dynamic.Interface {
	t.Helper()

	client, err := dynamic.NewForConfig(startAPIServer(t))
	if err != nil {
		t.Fatalf("dynamic.NewForConfig() error = %v", err)
	}
	return client
}

func startAPIServer(t *testing.T) *rest.Config {
	t.Helper()

	options, err := NewOptions()
	if err != nil {
		t.Fatalf("NewOptions() error = %v", err)
	}
	options.RecommendedOptions.CoreAPI = nil
	options.RecommendedOptions.Authentication = nil
	options.RecommendedOptions.Authorization = nil
	options.RecommendedOptions.Admission = nil
	options.RecommendedOptions.Features.EnablePriorityAndFairness = false

	return startAPIServerWithOptions(t, options)
}

// startAPIServerWithOptions 使用用例指定的框架选项，存储、证书和监听端口仍全部隔离。
// 返回前等待 readyz，通过 Cleanup 取消并等待服务退出，不连接部署环境的 etcd。
func startAPIServerWithOptions(t *testing.T, options *Options) *rest.Config {
	t.Helper()

	etcd := testserver.RunEtcd(t, nil)
	t.Cleanup(func() {
		if err := etcd.Close(); err != nil {
			t.Errorf("close etcd client: %v", err)
		}
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for API server: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	options.RecommendedOptions.Etcd.StorageConfig.Transport.ServerList = etcd.Endpoints()
	options.RecommendedOptions.SecureServing.Listener = listener
	options.RecommendedOptions.SecureServing.ServerCert.CertDirectory = ""

	config, err := options.Config()
	if err != nil {
		t.Fatalf("Options.Config() error = %v", err)
	}
	server, err := config.Complete().New()
	if err != nil {
		t.Fatalf("CompletedConfig.New() error = %v", err)
	}

	cert, _ := options.RecommendedOptions.SecureServing.ServerCert.GeneratedCert.CurrentCertKeyContent()
	restConfig := &rest.Config{
		Host:            "https://" + listener.Addr().String(),
		TLSClientConfig: rest.TLSClientConfig{CAData: cert},
	}
	httpClient, err := rest.HTTPClientFor(restConfig)
	if err != nil {
		t.Fatalf("rest.HTTPClientFor() error = %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	finished := make(chan struct{})
	var runErr error
	go func() {
		runErr = server.GenericAPIServer.PrepareRun().RunWithContext(ctx)
		close(finished)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-finished:
			if runErr != nil {
				t.Errorf("API server shutdown: %v", runErr)
			}
		case <-time.After(15 * time.Second):
			t.Error("API server did not stop within 15 seconds")
		}
	})

	readyContext, stopWaiting := context.WithTimeout(ctx, 30*time.Second)
	defer stopWaiting()
	for {
		request, err := http.NewRequestWithContext(readyContext, http.MethodGet, restConfig.Host+"/readyz", nil)
		if err != nil {
			t.Fatalf("build readiness request: %v", err)
		}
		response, err := httpClient.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				break
			}
		}
		select {
		case <-finished:
			t.Fatalf("API server stopped before readiness: %v", runErr)
		case <-readyContext.Done():
			t.Fatalf("API server readiness: %v (last request error: %v)", readyContext.Err(), err)
		case <-time.After(100 * time.Millisecond):
		}
	}

	return restConfig
}

func expectResourceEvent(
	t *testing.T,
	stream watch.Interface,
	eventType watch.EventType,
	name, resourceVersion string,
) {
	t.Helper()

	select {
	case event, ok := <-stream.ResultChan():
		if !ok {
			t.Fatal("resource watch closed before expected event")
		}
		resource, ok := event.Object.(*unstructured.Unstructured)
		if !ok {
			t.Fatalf("resource watch object = %T, want *unstructured.Unstructured", event.Object)
		}
		if event.Type != eventType || resource.GetName() != name {
			t.Errorf("resource watch event = %s %q, want %s %q",
				event.Type, resource.GetName(), eventType, name)
		}
		if resourceVersion != "" && resource.GetResourceVersion() != resourceVersion {
			t.Errorf("resource watch resourceVersion = %q, want %q",
				resource.GetResourceVersion(), resourceVersion)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %q %s event", name, eventType)
	}
}

func assertSpec(t *testing.T, resource *unstructured.Unstructured, want map[string]any) {
	t.Helper()

	got, found, err := unstructured.NestedMap(resource.Object, "spec")
	if err != nil || !found {
		t.Fatalf("read resource spec: found %t, error %v", found, err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("resource spec mismatch (-want +got):\n%s", diff)
	}
}

func assertConditions(t *testing.T, resource *unstructured.Unstructured, want int) {
	t.Helper()

	conditions, found, err := unstructured.NestedSlice(resource.Object, "status", "conditions")
	if resource.GetKind() == "HTTPRoute" || resource.GetKind() == "ModelRoute" || resource.GetKind() == "MCPRoute" {
		parents, exists, parentErr := unstructured.NestedSlice(resource.Object, "status", "parents")
		if parentErr != nil {
			t.Fatalf("read status.parents: %v", parentErr)
		}
		found = exists && len(parents) > 0
		if found {
			conditions, _, err = unstructured.NestedSlice(parents[0].(map[string]any), "conditions")
		}
	}
	if err != nil {
		t.Fatalf("read resource status.conditions: %v", err)
	}
	if len(conditions) != want || (!found && want > 0) {
		t.Errorf("resource status.conditions = %#v, want %d item(s)", conditions, want)
	}
}

func statusForKind(kind string, conditions []any) map[string]any {
	if kind == "HTTPRoute" || kind == "ModelRoute" || kind == "MCPRoute" {
		return map[string]any{"parents": []any{map[string]any{
			"parentRef":      map[string]any{"name": "edge", "sectionName": "http"},
			"controllerName": "gateway.ingate.io/test",
			"conditions":     conditions,
		}}}
	}
	return map[string]any{"conditions": conditions}
}

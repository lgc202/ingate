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
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage/etcd3/testserver"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// TestResourceHTTPContract 通过真实 HTTPS 和 etcd 验证三种资源的存储契约。
func TestResourceHTTPContract(t *testing.T) {
	tests := []struct {
		name, kind, resource string
		spec, updatedSpec    map[string]any
	}{
		{
			name: "edge", kind: "Gateway", resource: "gateways",
			spec: map[string]any{"listeners": []any{
				map[string]any{"name": "http", "protocol": "HTTP", "port": int64(8080)},
			}},
			updatedSpec: map[string]any{"listeners": []any{
				map[string]any{"name": "http", "protocol": "HTTP", "port": int64(8081)},
			}},
		},
		{
			name: "api", kind: "Route", resource: "routes",
			spec: map[string]any{
				"gatewayName": "edge", "listenerName": "http", "upstreamName": "backend",
				"path": map[string]any{"type": "PathPrefix", "value": "/api"},
			},
			updatedSpec: map[string]any{
				"gatewayName": "edge", "listenerName": "http", "upstreamName": "backend",
				"path": map[string]any{"type": "PathPrefix", "value": "/v1"},
			},
		},
		{
			name: "backend", kind: "Upstream", resource: "upstreams",
			spec: map[string]any{"endpoints": []any{
				map[string]any{"address": "127.0.0.1", "port": int64(8080), "weight": int64(1)},
			}},
			updatedSpec: map[string]any{"endpoints": []any{
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
		"spec": spec,
		"status": map[string]any{"conditions": []any{
			map[string]any{"type": "Accepted", "status": "True"},
		}},
	}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create(%s) error = %v", kind, err)
	}
	if created.GetGeneration() != 1 || created.GetResourceVersion() == "" {
		t.Fatalf("Create(%s) metadata = %#v, want generation 1 and resourceVersion", kind, created.Object["metadata"])
	}
	assertConditions(t, created, 0)

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

	stream, err := resources.Watch(ctx, metav1.ListOptions{ResourceVersion: created.GetResourceVersion()})
	if err != nil {
		t.Fatalf("Watch(%s) error = %v", kind, err)
	}
	defer stream.Stop()

	updated := created.DeepCopy()
	updated.Object["spec"] = updatedSpec
	updated.Object["status"] = map[string]any{"conditions": []any{
		map[string]any{"type": "Accepted", "status": "True"},
	}}
	updated, err = resources.Update(ctx, updated, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Update(%s spec) error = %v", kind, err)
	}
	if updated.GetGeneration() != 2 || updated.GetResourceVersion() == created.GetResourceVersion() {
		t.Errorf("Update(%s spec) metadata = %#v, want new resourceVersion and generation 2",
			kind, updated.Object["metadata"])
	}
	assertSpec(t, updated, updatedSpec)
	assertConditions(t, updated, 0)
	expectResourceEvent(t, stream, watch.Modified, name, updated.GetResourceVersion())

	if _, err := resources.Update(ctx, created.DeepCopy(), metav1.UpdateOptions{}); !apierrors.IsConflict(err) {
		t.Errorf("Update(%s, stale resourceVersion) error = %v, want Conflict", kind, err)
	}

	status := updated.DeepCopy()
	status.Object["spec"] = spec
	status.Object["status"] = map[string]any{"conditions": []any{
		map[string]any{
			"type": "Accepted", "status": "True", "reason": "Configured",
			"message": "ready", "observedGeneration": int64(2),
			"lastTransitionTime": metav1.Now().Format(time.RFC3339),
		},
	}}
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

	options, err := NewOptions()
	if err != nil {
		t.Fatalf("NewOptions() error = %v", err)
	}
	options.RecommendedOptions.Etcd.StorageConfig.Transport.ServerList = etcd.Endpoints()
	options.RecommendedOptions.SecureServing.Listener = listener
	options.RecommendedOptions.SecureServing.ServerCert.CertDirectory = ""
	options.RecommendedOptions.CoreAPI = nil
	options.RecommendedOptions.Authentication = nil
	options.RecommendedOptions.Authorization = nil
	options.RecommendedOptions.Admission = nil
	options.RecommendedOptions.Features.EnablePriorityAndFairness = false

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

	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		t.Fatalf("dynamic.NewForConfig() error = %v", err)
	}
	return client
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
	if err != nil {
		t.Fatalf("read resource status.conditions: %v", err)
	}
	if len(conditions) != want || (!found && want > 0) {
		t.Errorf("resource status.conditions = %#v, want %d item(s)", conditions, want)
	}
}

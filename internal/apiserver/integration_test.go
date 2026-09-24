//go:build integration

package apiserver

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage/etcd3/testserver"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

var gatewayResource = gatewayv1alpha1.SchemeGroupVersion.WithResource("gateways")

// TestGatewayHTTPContract 覆盖真实 HTTP 与 etcd 存储边界，而非直接调用 REST Strategy。
func TestGatewayHTTPContract(t *testing.T) {
	gateways := startGatewayAPI(t)
	ctx := t.Context()

	created, err := gateways.Create(ctx, &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.ingate.io/v1alpha1",
		"kind":       "Gateway",
		"metadata":   map[string]any{"name": "edge"},
		"spec": map[string]any{"listeners": []any{
			map[string]any{"name": "http", "protocol": "HTTP", "port": int64(8080)},
		}},
		"status": map[string]any{"conditions": []any{
			map[string]any{"type": "Accepted", "status": "True"},
		}},
	}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create(Gateway) error = %v", err)
	}
	if created.GetGeneration() != 1 || created.GetResourceVersion() == "" {
		t.Fatalf("created metadata = %#v, want generation 1 and resourceVersion", created.Object["metadata"])
	}
	assertConditions(t, created, 0)

	got, err := gateways.Get(ctx, "edge", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(Gateway) error = %v", err)
	}
	if got.GetResourceVersion() != created.GetResourceVersion() {
		t.Errorf("Get(Gateway) resourceVersion = %q, want %q", got.GetResourceVersion(), created.GetResourceVersion())
	}
	listed, err := gateways.List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("List(Gateway) error = %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].GetName() != "edge" {
		t.Fatalf("List(Gateway) items = %#v, want only edge", listed.Items)
	}

	stream, err := gateways.Watch(ctx, metav1.ListOptions{ResourceVersion: created.GetResourceVersion()})
	if err != nil {
		t.Fatalf("Watch(Gateway) error = %v", err)
	}
	defer stream.Stop()

	updated := created.DeepCopy()
	listeners, found, err := unstructured.NestedSlice(updated.Object, "spec", "listeners")
	if err != nil || !found || len(listeners) != 1 {
		t.Fatalf("read Gateway spec.listeners: found %t, items %d, error %v", found, len(listeners), err)
	}
	listeners[0].(map[string]any)["port"] = int64(8081)
	if err := unstructured.SetNestedSlice(updated.Object, listeners, "spec", "listeners"); err != nil {
		t.Fatalf("SetNestedSlice(listeners) error = %v", err)
	}
	updated.Object["status"] = map[string]any{"conditions": []any{
		map[string]any{"type": "Accepted", "status": "True"},
	}}
	updated, err = gateways.Update(ctx, updated, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Update(Gateway spec) error = %v", err)
	}
	if updated.GetGeneration() != 2 || updated.GetResourceVersion() == created.GetResourceVersion() {
		t.Errorf("updated metadata = %#v, want new resourceVersion and generation 2", updated.Object["metadata"])
	}
	assertConditions(t, updated, 0)
	expectGatewayEvent(t, stream, watch.Modified, updated.GetResourceVersion())

	stale := created.DeepCopy()
	if _, err := gateways.Update(ctx, stale, metav1.UpdateOptions{}); !apierrors.IsConflict(err) {
		t.Errorf("Update(Gateway stale resourceVersion) error = %v, want Conflict", err)
	}

	status := updated.DeepCopy()
	status.Object["spec"] = map[string]any{"listeners": []any{
		map[string]any{"name": "http", "protocol": "HTTP", "port": int64(9090)},
	}}
	status.Object["status"] = map[string]any{"conditions": []any{
		map[string]any{
			"type": "Accepted", "status": "True", "reason": "Configured",
			"message": "ready", "observedGeneration": int64(2),
			"lastTransitionTime": metav1.Now().Format(time.RFC3339),
		},
	}}
	status, err = gateways.UpdateStatus(ctx, status, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("UpdateStatus(Gateway) error = %v", err)
	}
	if status.GetGeneration() != 2 || listenerPort(t, status) != 8081 {
		t.Errorf("status update changed spec or generation: %#v", status.Object)
	}
	assertConditions(t, status, 1)
	expectGatewayEvent(t, stream, watch.Modified, status.GetResourceVersion())

	if err := gateways.Delete(ctx, "edge", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Delete(Gateway) error = %v", err)
	}
	expectGatewayEvent(t, stream, watch.Deleted, "")
	if _, err := gateways.Get(ctx, "edge", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Errorf("Get(deleted Gateway) error = %v, want NotFound", err)
	}
}

func startGatewayAPI(t *testing.T) dynamic.ResourceInterface {
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
	return client.Resource(gatewayResource)
}

func expectGatewayEvent(t *testing.T, stream watch.Interface, eventType watch.EventType, resourceVersion string) {
	t.Helper()

	select {
	case event, ok := <-stream.ResultChan():
		if !ok {
			t.Fatal("Gateway watch closed before expected event")
		}
		gateway, ok := event.Object.(*unstructured.Unstructured)
		if !ok {
			t.Fatalf("Gateway watch object = %T, want *unstructured.Unstructured", event.Object)
		}
		if event.Type != eventType || gateway.GetName() != "edge" {
			t.Errorf("Gateway watch event = %s %q, want %s edge", event.Type, gateway.GetName(), eventType)
		}
		if resourceVersion != "" && gateway.GetResourceVersion() != resourceVersion {
			t.Errorf("Gateway watch resourceVersion = %q, want %q", gateway.GetResourceVersion(), resourceVersion)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for Gateway %s event", eventType)
	}
}

func assertConditions(t *testing.T, gateway *unstructured.Unstructured, want int) {
	t.Helper()

	conditions, found, err := unstructured.NestedSlice(gateway.Object, "status", "conditions")
	if err != nil {
		t.Fatalf("read Gateway status.conditions: %v", err)
	}
	if len(conditions) != want || (!found && want > 0) {
		t.Errorf("Gateway status.conditions = %#v, want %d item(s)", conditions, want)
	}
}

func listenerPort(t *testing.T, gateway *unstructured.Unstructured) int64 {
	t.Helper()

	listeners, found, err := unstructured.NestedSlice(gateway.Object, "spec", "listeners")
	if err != nil || !found || len(listeners) != 1 {
		t.Fatalf("Gateway spec.listeners = %#v, found %t, error %v", listeners, found, err)
	}
	port, found, err := unstructured.NestedInt64(listeners[0].(map[string]any), "port")
	if err != nil || !found {
		t.Fatalf("Gateway spec.listeners[0].port: found %t, error %v", found, err)
	}
	return port
}

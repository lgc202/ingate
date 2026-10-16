package gateway

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestReconcileStoresAndRevokesAIConfiguration(t *testing.T) {
	snapshot := aiResourceSnapshot()
	route := routeResource("ordinary", 1, "public", "api", "model-network", gatewayv1alpha1.PathMatchPrefix)
	route.Namespace = "team-a"
	snapshot.httpRoutes.Items = append(snapshot.httpRoutes.Items, route)
	apiClient := aiResourceClient(t, snapshot)
	var published dataplane.Config
	r := newTestReconciler(t, apiClient, publisherFunc(func(_ context.Context, config dataplane.Config) error {
		published = config
		return nil
	}))
	want := aiConfigPayload(t, snapshot)

	if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
		t.Fatalf("Reconcile(HTTP, model and MCP resources) error = %v", err)
	}

	stored := storedAIConfigMap(t, apiClient)
	if stored.Data[aidataplane.ConfigMapDataKey] != string(want) {
		t.Errorf("Reconcile() AI payload = %s, want %s", stored.Data[aidataplane.ConfigMapDataKey], want)
	}

	wantHTTP := compile(resolve(snapshot.gateways.Items, snapshot.httpRoutes.Items, snapshot.backends.Items))
	if diff := cmp.Diff(wantHTTP, published); diff != "" {
		t.Errorf("Reconcile() HTTP payload mismatch (-want +got):\n%s", diff)
	}

	if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
		t.Fatalf("Reconcile(unchanged resources) error = %v", err)
	}

	if diff := cmp.Diff(stored, storedAIConfigMap(t, apiClient)); diff != "" {
		t.Errorf("Reconcile(unchanged resources) wrote ConfigMap (-want +got):\n%s", diff)
	}

	drifted := stored.DeepCopy()
	drifted.Data[aidataplane.ConfigMapDataKey] = string(aiConfigPayload(t, resources{}))
	if err := apiClient.Update(t.Context(), drifted); err != nil {
		t.Fatalf("Update(drifted AI ConfigMap) error = %v", err)
	}

	if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
		t.Fatalf("Reconcile(drifted AI ConfigMap) error = %v", err)
	}

	if got := storedAIConfigMap(t, apiClient).Data[aidataplane.ConfigMapDataKey]; got != string(want) {
		t.Errorf("Reconcile(drifted AI ConfigMap) payload = %s, want restored payload %s", got, want)
	}

	if err := apiClient.Delete(t.Context(), &snapshot.modelRoutes.Items[0]); err != nil {
		t.Fatalf("Delete(ModelRoute) error = %v", err)
	}

	if err := apiClient.Delete(t.Context(), &snapshot.mcpRoutes.Items[0]); err != nil {
		t.Fatalf("Delete(MCPRoute) error = %v", err)
	}

	if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
		t.Fatalf("Reconcile(no model or MCP routes) error = %v", err)
	}

	empty := aiConfigPayload(t, resources{})
	revoked := storedAIConfigMap(t, apiClient)
	if revoked.Data[aidataplane.ConfigMapDataKey] != string(empty) || revoked.ResourceVersion == stored.ResourceVersion {
		t.Errorf("Reconcile(no AI routes) ConfigMap = %+v, want complete empty payload with changed resourceVersion", revoked)
	}

	if diff := cmp.Diff(wantHTTP, published); diff != "" {
		t.Errorf("Reconcile(no AI routes) changed HTTP payload (-want +got):\n%s", diff)
	}

	if err := apiClient.Delete(t.Context(), &revoked); err != nil {
		t.Fatalf("Delete(AI ConfigMap) error = %v", err)
	}

	if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
		t.Fatalf("Reconcile(deleted AI ConfigMap) error = %v", err)
	}

	if got := storedAIConfigMap(t, apiClient).Data[aidataplane.ConfigMapDataKey]; got != string(empty) {
		t.Errorf("Reconcile(deleted AI ConfigMap) payload = %s, want restored complete empty payload %s", got, empty)
	}
}

func TestReconcileIsolatesResourceReadFailures(t *testing.T) {
	tests := []struct {
		name         string
		failedList   client.ObjectList
		wantHTTP     int
		wantAIWrites int
	}{
		{name: "common Gateway read", failedList: &gatewayv1alpha1.GatewayList{}},
		{name: "common Backend read", failedList: &gatewayv1alpha1.BackendList{}},
		{name: "HTTPRoute read", failedList: &gatewayv1alpha1.HTTPRouteList{}, wantAIWrites: 1},
		{name: "AIBackend read", failedList: &gatewayv1alpha1.AIBackendList{}, wantHTTP: 1},
		{name: "ModelRoute read", failedList: &gatewayv1alpha1.ModelRouteList{}, wantHTTP: 1},
		{name: "MCPRoute read", failedList: &gatewayv1alpha1.MCPRouteList{}, wantHTTP: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := aiResourceSnapshot()
			previous := managedAIConfigMap(string(aiConfigPayload(t, resources{})))
			apiClient := aiResourceClient(t, snapshot, &previous)
			before := storedAIConfigMap(t, apiClient)
			cause := errors.New("dependency unavailable")
			ctx := t.Context()
			var httpCalls, aiWrites int
			writes := interceptor.NewClient(apiClient, interceptor.Funcs{
				List: func(
					gotCtx context.Context,
					underlying client.WithWatch,
					list client.ObjectList,
					options ...client.ListOption,
				) error {
					if gotCtx != ctx {
						t.Error("List() did not receive the Reconcile context")
					}

					if reflect.TypeOf(list) == reflect.TypeOf(tt.failedList) {
						return cause
					}

					return underlying.List(gotCtx, list, options...)
				},
				Update: func(
					gotCtx context.Context,
					underlying client.WithWatch,
					object client.Object,
					options ...client.UpdateOption,
				) error {
					if _, isConfigMap := object.(*corev1.ConfigMap); !isConfigMap {
						t.Fatalf("unexpected main-resource Update(%T)", object)
					}

					aiWrites++

					return underlying.Update(gotCtx, object, options...)
				},
			})
			r := New(writes, apiClient, publisherFunc(func(context.Context, dataplane.Config) error {
				httpCalls++
				return nil
			}), "control-plane")

			_, err := r.Reconcile(ctx, sharedRequest())

			if !errors.Is(err, cause) {
				t.Errorf("Reconcile(%s) error = %v, want wrapped %v", tt.name, err, cause)
			}

			if httpCalls != tt.wantHTTP || aiWrites != tt.wantAIWrites {
				t.Errorf("Reconcile(%s) HTTP/AI attempts = %d/%d, want %d/%d", tt.name, httpCalls, aiWrites, tt.wantHTTP, tt.wantAIWrites)
			}

			got := storedAIConfigMap(t, apiClient)
			if tt.wantAIWrites == 0 {
				if diff := cmp.Diff(before, got); diff != "" {
					t.Errorf("Reconcile(%s) replaced previous AI configuration (-want +got):\n%s", tt.name, diff)
				}
			} else if got.Data[aidataplane.ConfigMapDataKey] != string(aiConfigPayload(t, snapshot)) {
				t.Errorf("Reconcile(%s) did not store the complete AI configuration: %s", tt.name, got.Data[aidataplane.ConfigMapDataKey])
			}
		})
	}
}

func TestReconcileStoresAIAfterHTTPPublishFailure(t *testing.T) {
	snapshot := aiResourceSnapshot()
	apiClient := aiResourceClient(t, snapshot)
	cause := errors.New("HTTP publish unavailable")
	r := newTestReconciler(t, apiClient, publisherFunc(func(context.Context, dataplane.Config) error { return cause }))

	_, err := r.Reconcile(t.Context(), sharedRequest())

	if !errors.Is(err, cause) {
		t.Errorf("Reconcile(HTTP publish failure) error = %v, want wrapped %v", err, cause)
	}

	if got, want := storedAIConfigMap(t, apiClient).Data[aidataplane.ConfigMapDataKey], string(aiConfigPayload(t, snapshot)); got != want {
		t.Errorf("Reconcile(HTTP publish failure) AI payload = %s, want %s", got, want)
	}
}

func TestReconcileStoresAIAfterHTTPStatusFailure(t *testing.T) {
	snapshot := aiResourceSnapshot()
	apiClient := aiResourceClient(t, snapshot)
	cause := errors.New("HTTP status unavailable")
	writes := interceptor.NewClient(apiClient, interceptor.Funcs{
		SubResourceUpdate: func(context.Context, client.Client, string, client.Object, ...client.SubResourceUpdateOption) error {
			return cause
		},
	})
	r := New(writes, apiClient, publisherFunc(func(context.Context, dataplane.Config) error { return nil }), "control-plane")

	_, err := r.Reconcile(t.Context(), sharedRequest())

	if !errors.Is(err, cause) {
		t.Errorf("Reconcile(HTTP status failure) error = %v, want wrapped %v", err, cause)
	}

	if got, want := storedAIConfigMap(t, apiClient).Data[aidataplane.ConfigMapDataKey], string(aiConfigPayload(t, snapshot)); got != want {
		t.Errorf("Reconcile(HTTP status failure) AI payload = %s, want %s", got, want)
	}
}

func TestReconcileKeepsAIConfigurationAfterWriteFailure(t *testing.T) {
	snapshot := aiResourceSnapshot()
	previous := managedAIConfigMap(string(aiConfigPayload(t, resources{})))
	apiClient := aiResourceClient(t, snapshot, &previous)
	cause := errors.New("AI write unavailable")
	writes := interceptor.NewClient(apiClient, interceptor.Funcs{
		Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error { return cause },
	})
	var httpCalls int
	r := New(writes, apiClient, publisherFunc(func(context.Context, dataplane.Config) error {
		httpCalls++
		return nil
	}), "control-plane")

	_, err := r.Reconcile(t.Context(), sharedRequest())

	if !errors.Is(err, cause) || httpCalls != 1 {
		t.Errorf("Reconcile(AI write failure) error/HTTP attempts = %v/%d, want wrapped %v and one HTTP publish", err, httpCalls, cause)
	}

	if diff := cmp.Diff(previous, storedAIConfigMap(t, apiClient)); diff != "" {
		t.Errorf("Reconcile(AI write failure) changed old ConfigMap (-want +got):\n%s", diff)
	}
}

func TestReconcileKeepsAIConfigurationAfterCompileFailure(t *testing.T) {
	snapshot := aiResourceSnapshot()
	snapshot.aiBackends.Items[0].Spec.Schema.Prefix = "%zz"
	previous := managedAIConfigMap(string(aiConfigPayload(t, resources{})))
	apiClient := aiResourceClient(t, snapshot, &previous)
	writes := forbiddenAIConfigWrites(t, apiClient)
	var httpCalls int
	r := New(writes, apiClient, publisherFunc(func(context.Context, dataplane.Config) error {
		httpCalls++
		return nil
	}), "control-plane")

	_, err := r.Reconcile(t.Context(), sharedRequest())

	if _, ok := errors.AsType[*url.Error](err); !ok || httpCalls != 1 {
		t.Errorf("Reconcile(invalid AI path) error/HTTP attempts = %v/%d, want wrapped URL error and one HTTP publish", err, httpCalls)
	}

	if diff := cmp.Diff(previous, storedAIConfigMap(t, apiClient)); diff != "" {
		t.Errorf("Reconcile(invalid AI path) replaced old ConfigMap (-want +got):\n%s", diff)
	}
}

func TestReconcileReturnsBothOutputErrors(t *testing.T) {
	snapshot := aiResourceSnapshot()
	previous := managedAIConfigMap(string(aiConfigPayload(t, resources{})))
	apiClient := aiResourceClient(t, snapshot, &previous)
	httpError := errors.New("HTTP publish unavailable")
	aiError := errors.New("AI write unavailable")
	writes := interceptor.NewClient(apiClient, interceptor.Funcs{
		Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error { return aiError },
	})
	r := New(writes, apiClient, publisherFunc(func(context.Context, dataplane.Config) error { return httpError }), "control-plane")

	_, err := r.Reconcile(t.Context(), sharedRequest())

	if !errors.Is(err, httpError) || !errors.Is(err, aiError) {
		t.Errorf("Reconcile(two failures) error = %v, want both %v and %v", err, httpError, aiError)
	}

	if diff := cmp.Diff(previous, storedAIConfigMap(t, apiClient)); diff != "" {
		t.Errorf("Reconcile(two failures) changed old ConfigMap (-want +got):\n%s", diff)
	}
}

func TestReconcileKeepsAIConfigurationAfterMCPModelConflict(t *testing.T) {
	snapshot := aiResourceSnapshot()
	snapshot.mcpRoutes.Items[0].Spec.Path = new(aidataplane.ModelResponsesPath)
	previous := managedAIConfigMap(string(aiConfigPayload(t, resources{})))
	apiClient := aiResourceClient(t, snapshot, &previous)
	writes := forbiddenAIConfigWrites(t, apiClient)
	var httpCalls, aiCalls int
	publisher := aiPublisherStub{
		publisherFunc: func(context.Context, dataplane.Config) error { httpCalls++; return nil },
		publishAI:     func(context.Context, aidataplane.Config) error { aiCalls++; return nil },
	}
	reconciler := New(writes, apiClient, publisher, "control-plane")

	_, err := reconciler.Reconcile(t.Context(), sharedRequest())

	if err == nil || httpCalls != 1 || aiCalls != 0 {
		t.Fatalf("Reconcile(conflict)=%v, HTTP/AI calls=%d/%d", err, httpCalls, aiCalls)
	}

	if diff := cmp.Diff(previous, storedAIConfigMap(t, apiClient)); diff != "" {
		t.Errorf("conflict replaced valid configuration (-want +got):\n%s", diff)
	}
}

func aiResourceClient(t *testing.T, snapshot resources, additional ...client.Object) client.WithWatch {
	t.Helper()

	var objects []client.Object
	for i := range snapshot.gateways.Items {
		objects = append(objects, &snapshot.gateways.Items[i])
	}

	for i := range snapshot.backends.Items {
		objects = append(objects, &snapshot.backends.Items[i])
	}

	for i := range snapshot.httpRoutes.Items {
		objects = append(objects, &snapshot.httpRoutes.Items[i])
	}

	for i := range snapshot.aiBackends.Items {
		objects = append(objects, &snapshot.aiBackends.Items[i])
	}

	for i := range snapshot.modelRoutes.Items {
		objects = append(objects, &snapshot.modelRoutes.Items[i])
	}

	for i := range snapshot.mcpRoutes.Items {
		objects = append(objects, &snapshot.mcpRoutes.Items[i])
	}

	objects = append(objects, additional...)

	return newFakeClient(t, objects...)
}

package gateway

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestAIConfigWriterCreatesAndRevokes(t *testing.T) {
	apiClient := aiConfigClient(t, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "control-plane"}})
	writer := aiConfigWriter{client: apiClient, reader: apiClient, namespace: "control-plane"}
	data := aiConfigPayload(t, aiResourceSnapshot())

	if err := writer.write(t.Context(), data); err != nil {
		t.Fatalf("write(initial configuration) error = %v", err)
	}

	got := storedAIConfigMap(t, apiClient)
	want := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "control-plane",
			Name:            aidataplane.ConfigMapName,
			Labels:          map[string]string{aiConfigManagedByKey: aiConfigManagedByValue},
			ResourceVersion: got.ResourceVersion,
		},
		Data: map[string]string{aidataplane.ConfigMapDataKey: string(data)},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("write(initial configuration) ConfigMap mismatch (-want +got):\n%s", diff)
	}

	if got.ResourceVersion == "" {
		t.Error("write(initial configuration) resourceVersion is empty")
	}

	empty := aiConfigPayload(t, resources{})
	if err := writer.write(t.Context(), empty); err != nil {
		t.Fatalf("write(empty configuration) error = %v", err)
	}

	got = storedAIConfigMap(t, apiClient)
	if got.Data[aidataplane.ConfigMapDataKey] != string(empty) || got.ResourceVersion == want.ResourceVersion {
		t.Errorf("write(empty configuration) ConfigMap = %+v, want complete empty payload and a new resourceVersion", got)
	}
}

func TestAIConfigWriterPreservesOtherFields(t *testing.T) {
	existing := managedAIConfigMap("old")
	existing.Labels["other"] = "preserved"
	existing.Annotations = map[string]string{"other": "preserved"}
	existing.Finalizers = []string{"example.com/preserve"}
	existing.OwnerReferences = []metav1.OwnerReference{{APIVersion: "v1", Kind: "Namespace", Name: "control-plane", UID: "owner"}}
	existing.Data["other"] = "preserved"
	existing.BinaryData = map[string][]byte{"other-binary": {1, 2, 3}}
	apiClient := aiConfigClient(t, &existing, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "control-plane"}})
	ctx := t.Context()
	var updates int
	writes := interceptor.NewClient(apiClient, interceptor.Funcs{
		Update: func(
			gotCtx context.Context,
			underlying client.WithWatch,
			object client.Object,
			options ...client.UpdateOption,
		) error {
			if gotCtx != ctx || object.GetResourceVersion() != existing.ResourceVersion {
				t.Errorf("Update() context/version = %v/%q, want original context and version %q", gotCtx, object.GetResourceVersion(), existing.ResourceVersion)
			}

			updates++

			return underlying.Update(gotCtx, object, options...)
		},
	})
	// 写客户端的 Get 禁止使用，证明读取来自独立的 APIReader，而非缓存客户端。
	writes = interceptor.NewClient(writes, interceptor.Funcs{
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			t.Fatal("write() read through the write client instead of APIReader")
			return nil
		},
	})
	reader := interceptor.NewClient(apiClient, interceptor.Funcs{
		Get: func(
			gotCtx context.Context,
			underlying client.WithWatch,
			key client.ObjectKey,
			object client.Object,
			options ...client.GetOption,
		) error {
			if gotCtx != ctx {
				t.Error("APIReader.Get() did not receive the write context")
			}

			return underlying.Get(gotCtx, key, object, options...)
		},
	})
	writer := aiConfigWriter{client: writes, reader: reader, namespace: "control-plane"}
	data := aiConfigPayload(t, aiResourceSnapshot())

	if err := writer.write(ctx, data); err != nil {
		t.Fatalf("write(changed configuration) error = %v", err)
	}

	got := storedAIConfigMap(t, apiClient)
	want := *existing.DeepCopy()
	want.Data[aidataplane.ConfigMapDataKey] = string(data)
	want.ResourceVersion = got.ResourceVersion
	if updates != 1 || got.ResourceVersion == existing.ResourceVersion {
		t.Errorf("write(changed configuration) updates/version = %d/%q, want one update and changed version", updates, got.ResourceVersion)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("write(changed configuration) replaced unrelated fields (-want +got):\n%s", diff)
	}
}

func TestAIConfigWriterSkipsUnchangedConfiguration(t *testing.T) {
	data := aiConfigPayload(t, aiResourceSnapshot())
	existing := managedAIConfigMap(string(data))
	existing.Immutable = new(true)
	apiClient := aiConfigClient(t, &existing, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "control-plane"}})
	writes := forbiddenAIConfigWrites(t, apiClient)
	writer := aiConfigWriter{client: writes, reader: apiClient, namespace: "control-plane"}

	if err := writer.write(t.Context(), data); err != nil {
		t.Fatalf("write(unchanged immutable configuration) error = %v", err)
	}

	if diff := cmp.Diff(existing, storedAIConfigMap(t, apiClient)); diff != "" {
		t.Errorf("write(unchanged configuration) changed stored object (-want +got):\n%s", diff)
	}
}

func TestAIConfigWriterRejectsUnsafeWrites(t *testing.T) {
	tests := []struct {
		name          string
		change        func(*corev1.Namespace, *corev1.ConfigMap)
		omitNamespace bool
		conflict      bool
	}{
		{name: "missing Namespace", omitNamespace: true},
		{
			name: "deleting Namespace",
			change: func(namespace *corev1.Namespace, _ *corev1.ConfigMap) {
				namespace.DeletionTimestamp = new(metav1.Unix(3, 0))
				namespace.Finalizers = []string{"example.com/preserve"}
			},
		},
		{
			name: "terminating Namespace",
			change: func(namespace *corev1.Namespace, _ *corev1.ConfigMap) {
				namespace.Status.Phase = corev1.NamespaceTerminating
			},
		},
		{
			name: "unmanaged ConfigMap",
			change: func(_ *corev1.Namespace, configMap *corev1.ConfigMap) {
				configMap.Labels = nil
			},
			conflict: true,
		},
		{
			name: "another owner",
			change: func(_ *corev1.Namespace, configMap *corev1.ConfigMap) {
				configMap.Labels[aiConfigManagedByKey] = "another-controller"
			},
			conflict: true,
		},
		{
			name: "deleting ConfigMap",
			change: func(_ *corev1.Namespace, configMap *corev1.ConfigMap) {
				configMap.DeletionTimestamp = new(metav1.Unix(3, 0))
				configMap.Finalizers = []string{"example.com/preserve"}
			},
		},
		{
			name: "immutable changed configuration",
			change: func(_ *corev1.Namespace, configMap *corev1.ConfigMap) {
				configMap.Immutable = new(true)
			},
		},
		{
			name: "binaryData occupies key",
			change: func(_ *corev1.Namespace, configMap *corev1.ConfigMap) {
				delete(configMap.Data, aidataplane.ConfigMapDataKey)
				configMap.BinaryData = map[string][]byte{aidataplane.ConfigMapDataKey: {1}}
			},
		},
		{
			name: "complete object exceeds limit",
			change: func(_ *corev1.Namespace, configMap *corev1.ConfigMap) {
				configMap.Data[aidataplane.ConfigMapDataKey] = "x"
				configMap.BinaryData = map[string][]byte{"other": make([]byte, corev1.MaxSecretSize-1)}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "control-plane"}}
			existing := managedAIConfigMap("old")
			if tt.change != nil {
				tt.change(&namespace, &existing)
			}

			objects := []client.Object{&existing}
			if !tt.omitNamespace {
				objects = append(objects, &namespace)
			}

			apiClient := aiConfigClient(t, objects...)
			before := storedAIConfigMap(t, apiClient)
			writer := aiConfigWriter{client: forbiddenAIConfigWrites(t, apiClient), reader: apiClient, namespace: "control-plane"}
			data := aiConfigPayload(t, aiResourceSnapshot())

			err := writer.write(t.Context(), data)
			if err == nil {
				t.Fatalf("write(%s) error = nil, want rejection", tt.name)
			}

			if tt.conflict && !apierrors.IsConflict(err) {
				t.Errorf("write(%s) error = %v, want Conflict", tt.name, err)
			}

			if tt.omitNamespace && !apierrors.IsNotFound(err) {
				t.Errorf("write(missing Namespace) error = %v, want wrapped NotFound", err)
			}

			if diff := cmp.Diff(before, storedAIConfigMap(t, apiClient)); diff != "" {
				t.Errorf("write(%s) changed stored object (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestAIConfigWriterAcceptsSizeLimit(t *testing.T) {
	data := aiConfigPayload(t, resources{})
	existing := managedAIConfigMap("old")
	existing.BinaryData = map[string][]byte{"other": make([]byte, corev1.MaxSecretSize-len(data))}
	apiClient := aiConfigClient(t, &existing, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "control-plane"}})
	writer := aiConfigWriter{client: apiClient, reader: apiClient, namespace: "control-plane"}

	if err := writer.write(t.Context(), data); err != nil {
		t.Fatalf("write(exact size limit) error = %v", err)
	}

	got := storedAIConfigMap(t, apiClient)
	if size := configMapDataSize(&got); size != corev1.MaxSecretSize {
		t.Errorf("write(exact size limit) data size = %d, want %d", size, corev1.MaxSecretSize)
	}

	if got.Data[aidataplane.ConfigMapDataKey] != string(data) {
		t.Errorf("write(exact size limit) payload = %q, want %q", got.Data[aidataplane.ConfigMapDataKey], data)
	}
}

func TestAIConfigWriterRejectsInvalidPayload(t *testing.T) {
	apiClient := aiConfigClient(t)
	tests := []struct {
		name string
		data []byte
	}{
		{name: "empty bytes"},
		{name: "oversized payload", data: []byte(strings.Repeat("x", corev1.MaxSecretSize+1))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := interceptor.NewClient(apiClient, interceptor.Funcs{
				Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
					t.Fatal("write(invalid arguments) performed a read")
					return nil
				},
			})
			writer := aiConfigWriter{client: forbiddenAIConfigWrites(t, apiClient), reader: reader, namespace: "control-plane"}

			if err := writer.write(t.Context(), tt.data); err == nil {
				t.Errorf("write(%s) error = nil, want rejection before API access", tt.name)
			}
		})
	}
}

func TestAIConfigWriterReturnsAPIErrors(t *testing.T) {
	conflict := apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, aidataplane.ConfigMapName, errors.New("stale version"))
	alreadyExists := apierrors.NewAlreadyExists(schema.GroupResource{Resource: "configmaps"}, aidataplane.ConfigMapName)
	tests := []struct {
		name     string
		stage    string
		cause    error
		existing bool
	}{
		{name: "Namespace read", stage: "Namespace", cause: context.Canceled},
		{name: "ConfigMap read", stage: "ConfigMap", cause: errors.New("read unavailable")},
		{name: "Create", stage: "Create", cause: alreadyExists},
		{name: "Update", stage: "Update", cause: conflict, existing: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := []client.Object{&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "control-plane"}}}
			if tt.existing {
				configMap := managedAIConfigMap("old")
				objects = append(objects, &configMap)
			}

			apiClient := aiConfigClient(t, objects...)
			var failures int
			reader := interceptor.NewClient(apiClient, interceptor.Funcs{
				Get: func(
					ctx context.Context,
					underlying client.WithWatch,
					key client.ObjectKey,
					object client.Object,
					options ...client.GetOption,
				) error {
					if (tt.stage == "Namespace" && key.Namespace == "") || (tt.stage == "ConfigMap" && key.Namespace != "") {
						failures++
						return tt.cause
					}

					return underlying.Get(ctx, key, object, options...)
				},
			})
			writes := interceptor.NewClient(apiClient, interceptor.Funcs{
				Create: func(context.Context, client.WithWatch, client.Object, ...client.CreateOption) error {
					if tt.stage != "Create" {
						t.Fatal("write() unexpectedly called Create after a read error")
					}

					failures++

					return tt.cause
				},
				Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error {
					if tt.stage != "Update" {
						t.Fatal("write() unexpectedly called Update")
					}

					failures++

					return tt.cause
				},
			})
			writer := aiConfigWriter{client: writes, reader: reader, namespace: "control-plane"}
			data := aiConfigPayload(t, aiResourceSnapshot())

			err := writer.write(t.Context(), data)

			if !errors.Is(err, tt.cause) || failures != 1 {
				t.Errorf("write(%s failure) error/failures = %v/%d, want wrapped %v and one attempt", tt.name, err, failures, tt.cause)
			}
		})
	}
}

func aiConfigClient(t *testing.T, objects ...client.Object) client.WithWatch {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("core AddToScheme() error = %v", err)
	}

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func managedAIConfigMap(data string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "control-plane",
			Name:            aidataplane.ConfigMapName,
			Labels:          map[string]string{aiConfigManagedByKey: aiConfigManagedByValue},
			ResourceVersion: "42",
		},
		Data: map[string]string{aidataplane.ConfigMapDataKey: data},
	}
}

func storedAIConfigMap(t *testing.T, apiClient client.Reader) corev1.ConfigMap {
	t.Helper()

	var configMap corev1.ConfigMap
	if err := apiClient.Get(t.Context(), client.ObjectKey{Namespace: "control-plane", Name: aidataplane.ConfigMapName}, &configMap); err != nil {
		t.Fatalf("Get(AI ConfigMap) error = %v", err)
	}

	return configMap
}

func aiConfigPayload(t *testing.T, snapshot resources) []byte {
	t.Helper()

	data, err := encodeTestAI(snapshot)
	if err != nil {
		t.Fatalf("encodeTestAI() error = %v", err)
	}

	return data
}

func forbiddenAIConfigWrites(t *testing.T, apiClient client.WithWatch) client.WithWatch {
	t.Helper()

	return interceptor.NewClient(apiClient, interceptor.Funcs{
		Create: func(context.Context, client.WithWatch, client.Object, ...client.CreateOption) error {
			t.Fatal("write() unexpectedly created an object")
			return nil
		},
		Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error {
			t.Fatal("write() unexpectedly updated an object")
			return nil
		},
		Delete: func(context.Context, client.WithWatch, client.Object, ...client.DeleteOption) error {
			t.Fatal("write() unexpectedly deleted an object")
			return nil
		},
	})
}

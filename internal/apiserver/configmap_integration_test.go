//go:build integration

package apiserver

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/utils/ptr"
)

// TestConfigMapHTTPContract 通过标准 core/v1 客户端、真实 HTTPS 和隔离 etcd 验证存储入口。
func TestConfigMapHTTPContract(t *testing.T) {
	client, err := typedcorev1.NewForConfig(startAPIServer(t))
	if err != nil {
		t.Fatalf("create core client error = %v", err)
	}

	ctx := t.Context()
	for _, namespace := range []string{"team-a", "team-b"} {
		input := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		if _, err := client.Namespaces().Create(ctx, input, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Create(Namespace %q) error = %v", namespace, err)
		}
	}

	resources := client.ConfigMaps("team-a")
	otherResources := client.ConfigMaps("team-b")
	initial := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "runtime", Labels: map[string]string{"owner": "platform"}},
		Data:       map[string]string{"config.json": "not parsed by the ConfigMap API"},
		BinaryData: map[string][]byte{"bytes": {0, 255}},
	}
	created, err := resources.Create(ctx, initial, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create(ConfigMap) error = %v", err)
	}
	if created.Namespace != "team-a" || created.UID == "" || created.ResourceVersion == "" || created.CreationTimestamp.IsZero() {
		t.Errorf("Create(ConfigMap) metadata = %+v, want server namespace/identity/version/time", created.ObjectMeta)
	}

	shadow, err := otherResources.Create(ctx, initial.DeepCopy(), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create(same ConfigMap name in team-b) error = %v", err)
	}

	for _, tt := range []struct {
		name, fieldSelector, labelSelector string
		wantCount                          int
	}{
		{
			name: "matching", wantCount: 1,
			fieldSelector: "metadata.name=runtime,metadata.namespace=team-a",
			labelSelector: "owner=platform",
		},
		{name: "other_name", fieldSelector: "metadata.name=other"},
		{name: "other_namespace", fieldSelector: "metadata.namespace=team-b"},
		{name: "other_label", labelSelector: "owner=other"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			list, err := resources.List(ctx, metav1.ListOptions{FieldSelector: tt.fieldSelector, LabelSelector: tt.labelSelector})
			if err != nil {
				t.Fatalf("List(ConfigMap %s) error = %v", tt.name, err)
			}
			if got := len(list.Items); got != tt.wantCount {
				t.Errorf("List(ConfigMap %s) count = %d, want %d", tt.name, got, tt.wantCount)
			}
		})
	}
	if _, err := resources.List(ctx, metav1.ListOptions{FieldSelector: "data.config.json=x"}); !apierrors.IsBadRequest(err) {
		t.Errorf("List(ConfigMap unsupported field) error = %v, want BadRequest", err)
	}

	stream, err := resources.Watch(ctx, metav1.ListOptions{
		ResourceVersion: created.ResourceVersion,
		FieldSelector:   "metadata.name=runtime",
	})
	if err != nil {
		t.Fatalf("Watch(ConfigMap) error = %v", err)
	}
	t.Cleanup(stream.Stop)

	// 不匹配名称的事件先于目标更新；若 Watch 未筛选，下面的首个事件断言就会失败。
	other := initial.DeepCopy()
	other.Name = "other"
	if _, err := resources.Create(ctx, other, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create(nonmatching ConfigMap) error = %v", err)
	}
	next := created.DeepCopy()
	next.Data["config.json"] = "updated"
	updated, err := resources.Update(ctx, next, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Update(ConfigMap) error = %v", err)
	}
	if updated.Generation != created.Generation || updated.ResourceVersion == created.ResourceVersion {
		t.Errorf("Update(ConfigMap) generation/version = %d/%q, want unchanged Generation and new version", updated.Generation, updated.ResourceVersion)
	}
	expectConfigMapEvent(t, stream, watch.Modified, updated.ResourceVersion)

	if _, err := resources.Update(ctx, created.DeepCopy(), metav1.UpdateOptions{}); !apierrors.IsConflict(err) {
		t.Errorf("Update(ConfigMap stale version) error = %v, want Conflict", err)
	}

	// 无条件 Update 是标准 ConfigMap 行为；未来 Controller 仍须显式携带并发版本。
	unconditional := updated.DeepCopy()
	unconditional.ResourceVersion = ""
	unconditional.Data["config.json"] = "unconditional"
	updated, err = resources.Update(ctx, unconditional, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Update(ConfigMap without version) error = %v, want nil", err)
	}
	expectConfigMapEvent(t, stream, watch.Modified, updated.ResourceVersion)

	patched, err := resources.Patch(ctx, initial.Name, types.MergePatchType, []byte(`{"data":{"config.json":"patched"}}`), metav1.PatchOptions{})
	if err != nil {
		t.Fatalf("Patch(ConfigMap) error = %v", err)
	}
	if patched.Data["config.json"] != "patched" || !cmp.Equal(patched.BinaryData, initial.BinaryData) {
		t.Errorf("Patch(ConfigMap) data/binaryData = %v/%v, want patched text and unchanged binary data", patched.Data, patched.BinaryData)
	}
	expectConfigMapEvent(t, stream, watch.Modified, patched.ResourceVersion)

	if err := resources.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: "owner=platform"}); err != nil {
		t.Fatalf("DeleteCollection(ConfigMap in team-a) error = %v", err)
	}
	expectConfigMapEvent(t, stream, watch.Deleted, "")
	if _, err := resources.Get(ctx, initial.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Errorf("Get(deleted ConfigMap) error = %v, want NotFound", err)
	}

	remaining, err := otherResources.Get(ctx, initial.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(ConfigMap in team-b) error = %v, want present", err)
	}
	if diff := cmp.Diff(shadow, remaining); diff != "" {
		t.Errorf("team-a writes changed ConfigMap in team-b (-want +got):\n%s", diff)
	}
}

func TestConfigMapHTTPValidationAndImmutable(t *testing.T) {
	client, err := typedcorev1.NewForConfig(startAPIServer(t))
	if err != nil {
		t.Fatalf("create core client error = %v", err)
	}

	resources := client.ConfigMaps("team-a")
	ctx := t.Context()
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	if _, err := client.Namespaces().Create(ctx, namespace, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create(Namespace) error = %v", err)
	}

	for _, tt := range []struct {
		name       string
		data       map[string]string
		binaryData map[string][]byte
	}{
		{name: "invalid-key", data: map[string]string{"bad/key": "value"}},
		{
			name:       "duplicate-key",
			data:       map[string]string{"key": "value"},
			binaryData: map[string][]byte{"key": {1}},
		},
		{
			name:       "over-limit",
			data:       map[string]string{"text": strings.Repeat("x", corev1.MaxSecretSize)},
			binaryData: map[string][]byte{"bytes": {0}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resources.Create(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: tt.name},
				Data:       tt.data,
				BinaryData: tt.binaryData,
			}, metav1.CreateOptions{})
			if !apierrors.IsInvalid(err) {
				t.Errorf("Create(ConfigMap %s) error = %v, want Invalid", tt.name, err)
			}
		})
	}

	missing := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "missing"}}
	if _, err := resources.Update(ctx, missing, metav1.UpdateOptions{}); !apierrors.IsNotFound(err) {
		t.Errorf("Update(missing ConfigMap) error = %v, want NotFound", err)
	}

	created, err := resources.Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "immutable"},
		Immutable:  ptr.To(true),
		Data:       map[string]string{"config.json": "original"},
		BinaryData: map[string][]byte{"bytes": {0, 255}},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create(immutable ConfigMap) error = %v", err)
	}
	for _, tt := range []struct {
		name   string
		mutate func(*corev1.ConfigMap)
	}{
		{name: "data", mutate: func(c *corev1.ConfigMap) { c.Data["config.json"] = "changed" }},
		{name: "binary_data", mutate: func(c *corev1.ConfigMap) { c.BinaryData["bytes"][0] = 1 }},
		{name: "immutable", mutate: func(c *corev1.ConfigMap) { c.Immutable = ptr.To(false) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			next := created.DeepCopy()
			tt.mutate(next)
			if _, err := resources.Update(ctx, next, metav1.UpdateOptions{}); !apierrors.IsInvalid(err) {
				t.Errorf("Update(immutable ConfigMap %s) error = %v, want Invalid", tt.name, err)
			}
		})
	}

	next := created.DeepCopy()
	next.Labels = map[string]string{"owner": "platform"}
	updated, err := resources.Update(ctx, next, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Update(immutable ConfigMap metadata) error = %v, want nil", err)
	}
	if !cmp.Equal(updated.Data, created.Data) || !cmp.Equal(updated.BinaryData, created.BinaryData) || updated.Labels["owner"] != "platform" {
		t.Errorf("Update(immutable ConfigMap metadata) = %+v, want unchanged data and updated label", updated)
	}
}

func expectConfigMapEvent(
	t *testing.T,
	stream watch.Interface,
	eventType watch.EventType,
	version string,
) {
	t.Helper()

	select {
	case event, ok := <-stream.ResultChan():
		if !ok {
			t.Fatal("ConfigMap watch closed before expected event")
		}
		configMap, ok := event.Object.(*corev1.ConfigMap)
		if !ok {
			t.Fatalf("ConfigMap watch object = %T, want *corev1.ConfigMap", event.Object)
		}
		if event.Type != eventType || configMap.Name != "runtime" || configMap.Namespace != "team-a" {
			t.Errorf("ConfigMap watch type/namespace/name = %s/%q/%q, want %s/team-a/runtime", event.Type, configMap.Namespace, configMap.Name, eventType)
		}
		if version != "" && configMap.ResourceVersion != version {
			t.Errorf("ConfigMap watch version = %q, want %q", configMap.ResourceVersion, version)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for ConfigMap %s event", eventType)
	}
}

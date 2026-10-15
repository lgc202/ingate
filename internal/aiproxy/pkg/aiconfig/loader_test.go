package aiconfig

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestNewLoader(t *testing.T) {
	apiClient := fake.NewClientset()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, namespace := range []string{"", "*", "Team-A", "team/a"} {
		if _, err := NewLoader(apiClient, namespace, logger); err == nil {
			t.Errorf("NewLoader(namespace %q) error = nil, want invalid namespace error", namespace)
		}
	}
	if _, err := NewLoader(nil, "control-plane", logger); err == nil {
		t.Error("NewLoader(nil client) error = nil, want missing client error")
	}
	if _, err := NewLoader(apiClient, "control-plane", nil); err == nil {
		t.Error("NewLoader(nil logger) error = nil, want missing logger error")
	}
	if _, err := NewLoader(apiClient, "control-plane", logger); err != nil {
		t.Errorf("NewLoader(valid dependencies) error = %v, want nil", err)
	}
	if actions := apiClient.Actions(); len(actions) != 0 {
		t.Errorf("NewLoader() API actions = %v, want none", actions)
	}
}

func TestReplaceAndRevoke(t *testing.T) {
	l := newTestLoader(t, fake.NewClientset())
	configMap := fixtureConfigMap(t)
	want, err := newConfig([]byte(configMap.Data[aidataplane.ConfigMapDataKey]))
	if err != nil {
		t.Fatalf("newConfig(fixture) error = %v", err)
	}

	if err := l.replace(configMap); err != nil {
		t.Fatalf("replace(initial valid configuration) error = %v", err)
	}

	_, initial := loaderState(l)
	if initial == nil {
		t.Fatal("replace(initial) has no snapshot, want valid configuration")
	}
	if diff := cmp.Diff(want, initial, cmp.AllowUnexported(Config{})); diff != "" {
		t.Errorf("replace(initial) mismatch (-want +got):\n%s", diff)
	}

	configMap.Data[aidataplane.ConfigMapDataKey] += "\n"
	if err := l.replace(configMap); err != nil {
		t.Fatalf("replace(same content with whitespace) error = %v", err)
	}
	if _, got := loaderState(l); got != initial {
		t.Error("replace(same content) replaced the existing snapshot, want reuse")
	}

	configMap.Data[aidataplane.ConfigMapDataKey] = emptyPayload(t)
	if err := l.replace(configMap); err != nil {
		t.Fatalf("replace(valid empty configuration) error = %v", err)
	}
	_, revoked := loaderState(l)
	if revoked == nil || revoked.Revision() == initial.Revision() {
		t.Errorf("replace(empty) = %+v, want a valid new snapshot without listeners", revoked)
	}
	wantEmpty, err := newConfig([]byte(configMap.Data[aidataplane.ConfigMapDataKey]))
	if err != nil {
		t.Fatalf("newConfig(empty configuration) error = %v", err)
	}
	if diff := cmp.Diff(wantEmpty, revoked, cmp.AllowUnexported(Config{})); diff != "" {
		t.Errorf("replace(empty) did not revoke complete contents (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(want, initial, cmp.AllowUnexported(Config{})); diff != "" {
		t.Errorf("replace(empty) mutated the previous snapshot (-want +got):\n%s", diff)
	}
}

func TestReplaceRejectsInvalidUpdates(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*corev1.ConfigMap)
	}{
		{name: "missing key", change: func(c *corev1.ConfigMap) { delete(c.Data, aidataplane.ConfigMapDataKey) }},
		{name: "empty bytes", change: func(c *corev1.ConfigMap) { c.Data[aidataplane.ConfigMapDataKey] = "" }},
		{name: "invalid JSON", change: func(c *corev1.ConfigMap) { c.Data[aidataplane.ConfigMapDataKey] = "{" }},
		{name: "unsupported format", change: func(c *corev1.ConfigMap) {
			c.Data[aidataplane.ConfigMapDataKey] = strings.Replace(c.Data[aidataplane.ConfigMapDataKey], `"v1"`, `"v2"`, 1)
		}},
		{name: "binary key", change: func(c *corev1.ConfigMap) {
			c.BinaryData = map[string][]byte{aidataplane.ConfigMapDataKey: {}}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			l := newTestLoader(t, fake.NewClientset())
			valid := fixtureConfigMap(t)
			want, err := newConfig([]byte(valid.Data[aidataplane.ConfigMapDataKey]))
			if err != nil {
				t.Fatalf("newConfig(valid fixture) error = %v", err)
			}

			invalid := valid.DeepCopy()
			tt.change(invalid)

			if err := l.replace(invalid); err == nil {
				t.Errorf("replace(initial %s) error = nil, want rejection", tt.name)
			}
			if _, got := loaderState(l); got != nil {
				t.Errorf("replace(initial %s) = %+v, want no snapshot", tt.name, got)
			}

			if err := l.replace(valid); err != nil {
				t.Fatalf("replace(valid recovery) error = %v", err)
			}
			_, previous := loaderState(l)
			if previous == nil {
				t.Fatal("replace(valid recovery) has no snapshot, want valid configuration")
			}

			if err := l.replace(invalid); err == nil {
				t.Errorf("replace(update %s) error = nil, want rejection", tt.name)
			}
			_, got := loaderState(l)
			if got != previous {
				t.Errorf("replace(update %s) snapshot = %p, want previous %p", tt.name, got, previous)
			}
			if diff := cmp.Diff(want, got, cmp.AllowUnexported(Config{})); diff != "" {
				t.Errorf("replace(update %s) changed snapshot contents (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestReplaceNewInstanceAndDeletion(t *testing.T) {
	l := newTestLoader(t, fake.NewClientset())
	previous := fixtureConfigMap(t)
	if err := l.replace(previous); err != nil {
		t.Fatalf("replace(previous instance) error = %v", err)
	}

	current := previous.DeepCopy()
	current.UID = "replacement"
	current.Data[aidataplane.ConfigMapDataKey] = "{}"

	if err := l.replace(current); err == nil {
		t.Error("replace(new invalid instance) error = nil, want rejection")
	}
	if uid, got := loaderState(l); uid != current.UID || got != nil {
		t.Errorf("replace(new invalid instance) = (%q, %+v), want (%q, nil)", uid, got, current.UID)
	}

	current.Data = previous.Data
	if err := l.replace(current); err != nil {
		t.Fatalf("replace(new valid instance) error = %v", err)
	}
	_, loaded := loaderState(l)
	if loaded == nil {
		t.Fatal("replace(new valid instance) has no snapshot, want valid configuration")
	}

	if err := l.remove(previous); err != nil {
		t.Fatalf("remove(previous instance) error = %v", err)
	}
	if _, got := loaderState(l); got != loaded {
		t.Error("remove(previous instance) removed the current snapshot")
	}

	if err := l.remove(current); err != nil {
		t.Fatalf("remove(current instance) error = %v", err)
	}
	if uid, got := loaderState(l); uid != "" || got != nil {
		t.Errorf("remove(current instance) = (%q, %+v), want no instance or snapshot", uid, got)
	}
}

func TestNonMatchingObjects(t *testing.T) {
	l := newTestLoader(t, fake.NewClientset())
	valid := fixtureConfigMap(t)
	if err := l.replace(valid); err != nil {
		t.Fatalf("replace(target) error = %v", err)
	}
	_, initial := loaderState(l)
	if initial == nil {
		t.Fatal("replace(target) has no snapshot, want valid configuration")
	}
	for _, tt := range []struct {
		name      string
		namespace string
		object    string
	}{
		{name: "other namespace", namespace: "team-a", object: aidataplane.ConfigMapName},
		{name: "other name", namespace: "control-plane", object: "unrelated"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			unrelated := valid.DeepCopy()
			unrelated.Namespace = tt.namespace
			unrelated.Name = tt.object
			unrelated.Data[aidataplane.ConfigMapDataKey] = "invalid"

			if err := l.replace(unrelated); err != nil {
				t.Errorf("replace(%s) error = %v, want ignored", tt.name, err)
			}
			if err := l.remove(unrelated); err != nil {
				t.Errorf("remove(%s) error = %v, want ignored", tt.name, err)
			}
			if _, got := loaderState(l); got != initial {
				t.Errorf("events for %s changed snapshot, want original target snapshot", tt.name)
			}
		})
	}
}

func TestReplacePreservesErrorCause(t *testing.T) {
	l := newTestLoader(t, fake.NewClientset())
	configMap := fixtureConfigMap(t)
	valid, err := aidataplane.Decode([]byte(configMap.Data[aidataplane.ConfigMapDataKey]))
	if err != nil {
		t.Fatalf("Decode(fixture) error = %v", err)
	}
	configMap.Data[aidataplane.ConfigMapDataKey] = strings.Replace(
		configMap.Data[aidataplane.ConfigMapDataKey], valid.Revision, strings.Repeat("z", 64), 1,
	)

	var invalidByte hex.InvalidByteError
	err = l.replace(configMap)
	if !errors.As(err, &invalidByte) {
		t.Errorf("replace(invalid revision) error = %v, want wrapped hex.InvalidByteError", err)
	}
}

func TestEventsWithoutObjectIdentity(t *testing.T) {
	l := newTestLoader(t, fake.NewClientset())
	valid := fixtureConfigMap(t)
	if err := l.replace(valid); err != nil {
		t.Fatalf("replace(valid instance) error = %v", err)
	}
	_, initial := loaderState(l)
	missingUID := valid.DeepCopy()
	missingUID.UID = ""

	if err := l.replace(nil); err == nil {
		t.Error("replace(nil object) error = nil, want missing object error")
	}
	if err := l.replace(missingUID); err == nil {
		t.Error("replace(missing UID) error = nil, want missing identity error")
	}
	if err := l.remove(nil); err == nil {
		t.Error("remove(nil object) error = nil, want missing identity error")
	}
	if err := l.remove(missingUID); err == nil {
		t.Error("remove(missing UID) error = nil, want missing identity error")
	}
	if _, got := loaderState(l); got != initial {
		t.Error("events without object identity changed the snapshot, want retain previous configuration")
	}
}

func TestRunLoadsUpdatesDeletesAndStops(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		apiClient := fake.NewClientset()
		configMaps := apiClient.CoreV1().ConfigMaps("control-plane")
		l, cancel := runTestLoader(t, apiClient)
		if _, got := loaderState(l); got != nil {
			t.Errorf("Run(initial missing object) = %+v, want no snapshot", got)
		}

		configMap := fixtureConfigMap(t)
		configMap, err := configMaps.Create(t.Context(), configMap, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Create(target) error = %v", err)
		}
		synctest.Wait()

		_, initial := loaderState(l)
		if initial == nil {
			t.Fatalf("Run(created valid object) = %+v, want nonempty valid configuration", initial)
		}
		want, err := newConfig([]byte(configMap.Data[aidataplane.ConfigMapDataKey]))
		if err != nil {
			t.Fatalf("newConfig(initial configuration) error = %v", err)
		}

		configMap.Data[aidataplane.ConfigMapDataKey] = "{}"
		configMap.ResourceVersion = "2"

		if _, err := configMaps.Update(t.Context(), configMap, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("Update(invalid configuration) error = %v", err)
		}
		synctest.Wait()

		_, afterInvalid := loaderState(l)
		if afterInvalid != initial {
			t.Error("Run(invalid update) replaced the valid snapshot")
		}
		if diff := cmp.Diff(want, afterInvalid, cmp.AllowUnexported(Config{})); diff != "" {
			t.Errorf("Run(invalid update) changed snapshot contents (-want +got):\n%s", diff)
		}

		configMap.Data[aidataplane.ConfigMapDataKey] = emptyPayload(t)
		configMap.ResourceVersion = "3"

		if _, err := configMaps.Update(t.Context(), configMap, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("Update(empty configuration) error = %v", err)
		}
		synctest.Wait()

		if _, got := loaderState(l); got == nil || got.Revision() == initial.Revision() {
			t.Errorf("Run(empty update) = %+v, want valid empty snapshot", got)
		}
		wantEmpty, err := newConfig([]byte(configMap.Data[aidataplane.ConfigMapDataKey]))
		if err != nil {
			t.Fatalf("newConfig(empty configuration) error = %v", err)
		}
		_, empty := loaderState(l)
		if diff := cmp.Diff(wantEmpty, empty, cmp.AllowUnexported(Config{})); diff != "" {
			t.Errorf("Run(empty update) did not revoke complete contents (-want +got):\n%s", diff)
		}

		if err := configMaps.Delete(t.Context(), aidataplane.ConfigMapName, metav1.DeleteOptions{}); err != nil {
			t.Fatalf("Delete(target) error = %v", err)
		}
		synctest.Wait()

		if _, got := loaderState(l); got != nil {
			t.Errorf("Run(deleted target) = %+v, want no snapshot", got)
		}

		recreated := fixtureConfigMap(t)
		recreated.UID = "recreated"

		if _, err := configMaps.Create(t.Context(), recreated, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Create(recreated target) error = %v", err)
		}
		synctest.Wait()

		if uid, got := loaderState(l); uid != recreated.UID || got == nil {
			t.Errorf("Run(recreated target) = (%q, %+v), want recovered configuration", uid, got)
		}

		if err := l.Run(t.Context()); err == nil {
			t.Error("Run(already running) error = nil, want rejection")
		}

		cancel()
		synctest.Wait()

		if _, got := loaderState(l); got != nil {
			t.Errorf("Run(stopped) = %+v, want no snapshot", got)
		}

		if err := l.Run(t.Context()); err == nil {
			t.Error("Run(restart) error = nil, want rejection")
		}

		for _, action := range apiClient.Actions() {
			var selector string

			switch action := action.(type) {
			case clienttesting.ListAction:
				selector = action.GetListRestrictions().Fields.String()
			case clienttesting.WatchAction:
				selector = action.GetWatchRestrictions().Fields.String()
			default:
				continue
			}

			resource := action.GetResource()
			if resource.Group != "" || resource.Version != "v1" || resource.Resource != "configmaps" {
				t.Errorf("Run() %s resource = %v, want only core/v1 ConfigMaps", action.GetVerb(), resource)
			}
			if action.GetNamespace() != "control-plane" || selector != "metadata.name="+aidataplane.ConfigMapName {
				t.Errorf("Run() %s scope = (%q, %q), want only target namespace/name", action.GetVerb(), action.GetNamespace(), selector)
			}
		}
	})
}

func TestRunRelistFailureAndConfirmedAbsence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		configMap := fixtureConfigMap(t)
		apiClient := fake.NewClientset()
		stream := watch.NewRaceFreeFake()
		forbidden := apierrors.NewForbidden(schema.GroupResource{Resource: "configmaps"}, aidataplane.ConfigMapName, errors.New("denied"))
		var (
			mu                        sync.Mutex
			absent, failList          bool
			failedLists, missingLists int
		)
		apiClient.PrependReactor("list", "configmaps", func(clienttesting.Action) (bool, runtime.Object, error) {
			mu.Lock()
			defer mu.Unlock()
			if failList {
				failedLists++
				return true, nil, forbidden
			}
			objects := &corev1.ConfigMapList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}}
			if !absent {
				objects.Items = []corev1.ConfigMap{*configMap}
			} else {
				missingLists++
			}
			return true, objects, nil
		})
		apiClient.PrependWatchReactor("configmaps", func(clienttesting.Action) (bool, watch.Interface, error) {
			mu.Lock()
			defer mu.Unlock()
			return true, stream, nil
		})
		l, _ := runTestLoader(t, apiClient)
		_, initial := loaderState(l)
		if initial == nil {
			t.Fatal("Run(initial List) has no configuration, want valid snapshot")
		}

		mu.Lock()
		failList = true
		mu.Unlock()
		stream.Error(&metav1.Status{Code: 410, Reason: metav1.StatusReasonExpired})
		time.Sleep(10 * time.Second) // synctest 的虚拟时间，让 SDK 重试失败的 List。
		synctest.Wait()

		mu.Lock()
		failedAttempts := failedLists
		mu.Unlock()
		if failedAttempts == 0 {
			t.Fatal("Run(failed relist) made no failing List request, want exercised failure path")
		}
		if _, got := loaderState(l); got != initial {
			t.Error("Run(failed relist) cleared the configuration, want retain previous snapshot")
		}

		mu.Lock()
		failList = false
		absent = true
		stream = watch.NewRaceFreeFake()
		mu.Unlock()
		time.Sleep(60 * time.Second) // 推进 SDK 退避，不新增生产侧重试或轮询。
		synctest.Wait()

		mu.Lock()
		missingAttempts := missingLists
		mu.Unlock()
		if missingAttempts == 0 {
			t.Fatal("Run(successful relist) made no List request confirming absence")
		}
		if _, got := loaderState(l); got != nil {
			t.Errorf("Run(successful relist without target) = %+v, want no snapshot", got)
		}
	})
}

func TestRunWatchFailureRetainsConfig(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		apiClient := fake.NewClientset(fixtureConfigMap(t))
		forbidden := apierrors.NewForbidden(schema.GroupResource{Resource: "configmaps"}, aidataplane.ConfigMapName, errors.New("denied"))
		var watchAttempts atomic.Int32
		apiClient.PrependWatchReactor("configmaps", func(clienttesting.Action) (bool, watch.Interface, error) {
			watchAttempts.Add(1)
			return true, nil, forbidden
		})
		l, _ := runTestLoader(t, apiClient)

		if watchAttempts.Load() == 0 {
			t.Fatal("Run(watch failure) made no failing Watch request, want exercised failure path")
		}
		_, got := loaderState(l)
		if got == nil {
			t.Errorf("Run(watch failure) = %+v, want retain the valid initial List configuration", got)
		} else if _, err := got.ResolveModel(8080, "gateway/team-a/public/listener/api", "coding"); err != nil {
			t.Errorf("ResolveModel(after Watch failure) error = %v, want valid initial model route", err)
		}
	})
}

func TestRunCancelledBeforeStart(t *testing.T) {
	apiClient := fake.NewClientset()
	l := newTestLoader(t, apiClient)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := l.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Run(cancelled context) error = %v, want context.Canceled", err)
	}
	if actions := apiClient.Actions(); len(actions) != 0 {
		t.Errorf("Run(cancelled context) API actions = %v, want none", actions)
	}
}

func newTestLoader(t *testing.T, apiClient *fake.Clientset) *Loader {
	t.Helper()
	l, err := NewLoader(apiClient, "control-plane", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewLoader(test loader) error = %v", err)
	}

	return l
}

// runTestLoader 必须在 synctest 内调用；清理时取消并等待 Run 返回。
func runTestLoader(t *testing.T, apiClient *fake.Clientset) (*Loader, context.CancelFunc) {
	t.Helper()
	l := newTestLoader(t, apiClient)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- l.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("Run(stopped test loader) error = %v, want context.Canceled", err)
		}
	})
	synctest.Wait()

	return l, cancel
}

func fixtureConfigMap(t *testing.T) *corev1.ConfigMap {
	t.Helper()
	data, err := os.ReadFile("../../../pkg/aidataplane/testdata/v1.json")
	if err != nil {
		t.Fatalf("ReadFile(AI configuration fixture) error = %v", err)
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

func emptyPayload(t *testing.T) string {
	t.Helper()
	data, err := aidataplane.Encode(aidataplane.Config{
		FormatVersion:   aidataplane.FormatVersionV1,
		Listeners:       []aidataplane.Listener{},
		Backends:        []aidataplane.Backend{},
		NetworkBackends: []aidataplane.NetworkBackend{},
	})
	if err != nil {
		t.Fatalf("Encode(empty configuration) error = %v", err)
	}

	return string(data)
}

// loaderState 仅供测试观察私有状态，不提供可变配置的生产访问入口。
func loaderState(l *Loader) (types.UID, *Config) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.uid, l.config
}

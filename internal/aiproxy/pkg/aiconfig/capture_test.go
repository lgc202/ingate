package aiconfig

import (
	"errors"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/google/go-cmp/cmp"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestCaptureConfigurationChanges(t *testing.T) {
	l := newTestLoader(t, fake.NewClientset())
	if config, loaded := l.Capture(); loaded || config != nil {
		t.Fatalf("Capture(before load) = (%p, %t), want nil and false", config, loaded)
	}

	configMap := fixtureConfigMap(t)
	if err := l.replace(configMap); err != nil {
		t.Fatalf("replace(initial configuration) error = %v", err)
	}
	previous, loaded := l.Capture()
	if !loaded || previous == nil {
		t.Fatal("Capture(initial configuration) has no runtime configuration")
	}
	before, err := previous.ResolveModel(8080, "gateway/team-a/public/listener/api", "coding")
	if err != nil {
		t.Fatalf("ResolveModel(initial configuration) error = %v", err)
	}
	oldRevision := previous.Revision()
	compiled, err := aidataplane.Decode([]byte(configMap.Data[aidataplane.ConfigMapDataKey]))
	if err != nil {
		t.Fatalf("Decode(initial configuration) error = %v", err)
	}
	compiled.Backends[0].Schema.Prefix = "v2/"
	compiled.Backends[0].ModelNameOverride = new("code-v3")
	data, err := aidataplane.Encode(*compiled)
	if err != nil {
		t.Fatalf("Encode(updated configuration) error = %v", err)
	}
	configMap.Data[aidataplane.ConfigMapDataKey] = string(data)

	if err := l.replace(configMap); err != nil {
		t.Fatalf("replace(updated configuration) error = %v", err)
	}
	current, loaded := l.Capture()
	if !loaded || current == nil || current == previous || current.Revision() == oldRevision {
		t.Fatalf("Capture(updated configuration) = (%p, %t), want a new configuration and revision", current, loaded)
	}
	got, err := current.ResolveModel(8080, "gateway/team-a/public/listener/api", "coding")
	if err != nil {
		t.Fatalf("ResolveModel(updated configuration) error = %v", err)
	}

	want := before
	want.Schema.Prefix = "v2/"
	want.ModelNameOverride = "code-v3"
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Capture(updated configuration) mismatch (-want +got):\n%s", diff)
	}

	stillPrevious, err := previous.ResolveModel(8080, "gateway/team-a/public/listener/api", "coding")
	if err != nil {
		t.Fatalf("ResolveModel(previous captured configuration) error = %v", err)
	}
	if diff := cmp.Diff(before, stillPrevious); diff != "" {
		t.Errorf("replace() mutated an already captured configuration (-want +got):\n%s", diff)
	}
	if previous.Revision() != oldRevision {
		t.Errorf("previous.Revision() = %q, want %q", previous.Revision(), oldRevision)
	}

	configMap.Data[aidataplane.ConfigMapDataKey] = "{}"
	if err := l.replace(configMap); err == nil {
		t.Fatal("replace(invalid same instance) error = nil, want rejection")
	}
	if got, loaded := l.Capture(); !loaded || got != current {
		t.Errorf("Capture(invalid same instance) = (%p, %t), want previous valid %p", got, loaded, current)
	}

	configMap.Data[aidataplane.ConfigMapDataKey] = emptyPayload(t)
	if err := l.replace(configMap); err != nil {
		t.Fatalf("replace(empty configuration) error = %v", err)
	}
	empty, loaded := l.Capture()
	if !loaded || empty == nil {
		t.Fatal("Capture(empty configuration) has no valid empty configuration")
	}
	got, err = empty.ResolveModel(8080, "gateway/team-a/public/listener/api", "coding")
	if !errors.Is(err, ErrModelRouteNotFound) || got != (ModelBackend{}) {
		t.Errorf("ResolveModel(empty configuration) = (%+v, %v), want no route", got, err)
	}

	if err := l.remove(configMap); err != nil {
		t.Fatalf("remove(current instance) error = %v", err)
	}
	if got, loaded := l.Capture(); loaded || got != nil {
		t.Errorf("Capture(deleted instance) = (%p, %t), want nil and false", got, loaded)
	}

	configMap.UID = "replacement"
	configMap.Data[aidataplane.ConfigMapDataKey] = string(data)
	if err := l.replace(configMap); err != nil {
		t.Fatalf("replace(recreated valid configuration) error = %v", err)
	}
	configMap.UID = "invalid-replacement"
	configMap.Data[aidataplane.ConfigMapDataKey] = "{}"
	if err := l.replace(configMap); err == nil {
		t.Fatal("replace(invalid new instance) error = nil, want rejection")
	}
	if got, loaded := l.Capture(); loaded || got != nil {
		t.Errorf("Capture(invalid new instance) = (%p, %t), want nil and false", got, loaded)
	}
}

func TestCaptureRunLifecycle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l, cancel := runTestLoader(t, fake.NewClientset(fixtureConfigMap(t)))
		captured, loaded := l.Capture()
		if !loaded || captured == nil {
			t.Fatal("Capture(after initial List) has no runtime configuration")
		}
		if _, err := captured.ResolveModel(8080, "gateway/team-a/public/listener/api", "coding"); err != nil {
			t.Fatalf("ResolveModel(after initial List) error = %v", err)
		}

		cancel()
		synctest.Wait()

		if got, loaded := l.Capture(); loaded || got != nil {
			t.Errorf("Capture(after Run stopped) = (%p, %t), want nil and false", got, loaded)
		}
		if _, err := captured.ResolveModel(8080, "gateway/team-a/public/listener/api", "coding"); err != nil {
			t.Errorf("ResolveModel(previously captured configuration) error = %v, want unchanged configuration", err)
		}
	})
}

func TestCaptureConcurrentUpdates(t *testing.T) {
	l := newTestLoader(t, fake.NewClientset())
	first := fixtureConfigMap(t)
	second := first.DeepCopy()
	compiled, err := aidataplane.Decode([]byte(first.Data[aidataplane.ConfigMapDataKey]))
	if err != nil {
		t.Fatalf("Decode(first configuration) error = %v", err)
	}
	compiled.Backends[0].Schema.Prefix = "v2/"
	compiled.Backends[0].ModelNameOverride = new("code-v3")
	data, err := aidataplane.Encode(*compiled)
	if err != nil {
		t.Fatalf("Encode(second configuration) error = %v", err)
	}
	second.Data[aidataplane.ConfigMapDataKey] = string(data)
	if err := l.replace(first); err != nil {
		t.Fatalf("replace(first configuration) error = %v", err)
	}
	initial, loaded := l.Capture()
	if !loaded {
		t.Fatal("Capture(first configuration) has no configuration")
	}
	initialRevision := initial.Revision()
	updated, err := aidataplane.Decode(data)
	if err != nil {
		t.Fatalf("Decode(second configuration) error = %v", err)
	}

	var workers sync.WaitGroup
	workers.Go(func() {
		for range 50 {
			if err := l.replace(second); err != nil {
				t.Errorf("replace(second configuration) error = %v", err)
				return
			}
			if err := l.replace(first); err != nil {
				t.Errorf("replace(first configuration) error = %v", err)
				return
			}
		}
	})
	for range 4 {
		workers.Go(func() {
			for range 200 {
				captured, loaded := l.Capture()
				if !loaded || captured == nil {
					t.Error("Capture(during valid updates) has no configuration")
					return
				}
				got, err := captured.ResolveModel(8080, "gateway/team-a/public/listener/api", "coding")
				if err != nil {
					t.Errorf("ResolveModel(during valid updates) error = %v", err)
					return
				}

				wantPrefix, wantModel := "/v1", "code-v2"

				switch captured.Revision() {
				case initialRevision:
				case updated.Revision:
					wantPrefix, wantModel = "v2/", "code-v3"
				default:
					t.Errorf("Capture() revision = %q, want one of two published revisions", captured.Revision())
					return
				}
				if got.Schema.Prefix != wantPrefix || got.ModelNameOverride != wantModel {
					t.Errorf("ResolveModel(revision %s) = %+v, want prefix %q and model %q",
						captured.Revision(), got, wantPrefix, wantModel)
				}
			}
		})
	}
	workers.Wait()
}

package configmap

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	coreapi "github.com/lgc202/ingate/pkg/apis/core"
)

func TestStrategyScopeAndUpdatePolicy(t *testing.T) {
	strategy := NewStrategy(runtime.NewScheme())
	if !strategy.NamespaceScoped() || strategy.AllowCreateOnUpdate(t.Context()) || !strategy.AllowUnconditionalUpdate(t.Context()) {
		t.Errorf("ConfigMap strategy scope/create-on-update/unconditional = %t/%t/%t, want true/false/true",
			strategy.NamespaceScoped(), strategy.AllowCreateOnUpdate(t.Context()), strategy.AllowUnconditionalUpdate(t.Context()))
	}
}

func TestStrategyPreservesStandardConfigMapFields(t *testing.T) {
	strategy := NewStrategy(runtime.NewScheme())
	next := &coreapi.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "runtime", Generation: 7},
		Immutable:  ptr.To(true),
		Data:       map[string]string{"config.json": "{}"},
	}
	want := next.DeepCopy()
	strategy.PrepareForCreate(t.Context(), next)
	strategy.Canonicalize(next)
	if diff := cmp.Diff(want, next); diff != "" {
		t.Errorf("ConfigMap create preparation changed data/metadata (-want +got):\n%s", diff)
	}

	previous := next.DeepCopy()
	previous.Immutable = nil
	previous.Data["config.json"] = "previous"
	strategy.PrepareForUpdate(t.Context(), next, previous)
	if diff := cmp.Diff(want, next); diff != "" {
		t.Errorf("ConfigMap update preparation changed data/Generation (-want +got):\n%s", diff)
	}
}

func TestStrategyValidation(t *testing.T) {
	strategy := NewStrategy(runtime.NewScheme())
	next := &coreapi.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "runtime", ResourceVersion: "1"},
		Immutable:  ptr.To(true),
		Data:       map[string]string{"config.json": "{}"},
	}
	if errs := strategy.Validate(t.Context(), next); len(errs) != 0 {
		t.Errorf("ConfigMap strategy Validate() errors = %v, want valid", errs)
	}

	previous := next.DeepCopy()
	previous.Immutable = nil
	if errs := strategy.ValidateUpdate(t.Context(), next, previous); len(errs) != 0 {
		t.Errorf("ConfigMap strategy ValidateUpdate(mutable previous) errors = %v, want valid", errs)
	}

	invalid := next.DeepCopy()
	invalid.Data["invalid/key"] = "value"
	if errs := strategy.Validate(t.Context(), invalid); len(errs) == 0 {
		t.Error("ConfigMap strategy Validate(invalid key) errors = empty, want invalid")
	}

	invalid = next.DeepCopy()
	invalid.Data["config.json"] = "changed"
	if errs := strategy.ValidateUpdate(t.Context(), invalid, next); len(errs) == 0 {
		t.Error("ConfigMap strategy ValidateUpdate(immutable data) errors = empty, want forbidden")
	}
}

func TestRESTShortNames(t *testing.T) {
	if got := (&REST{}).ShortNames(); !cmp.Equal(got, []string{"cm"}) {
		t.Errorf("ConfigMap.ShortNames() = %v, want [cm]", got)
	}
}

package target_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lgc202/ingate/internal/core/ir"
	"github.com/lgc202/ingate/internal/core/runtime"
	"github.com/lgc202/ingate/internal/core/target"
	"github.com/lgc202/ingate/internal/core/target/debug"
	"github.com/lgc202/ingate/internal/core/target/xds"
)

func TestRegistryGet(t *testing.T) {
	registry, err := target.NewRegistry(debug.Translator{}, xds.Translator{})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	translator, ok := registry.Get("xds")
	if !ok {
		t.Fatal(`Get("xds") ok = false`)
	}
	if translator.Target() != "xds" {
		t.Fatalf("Target() = %q, want xds", translator.Target())
	}
}

func TestRegistryGetMissing(t *testing.T) {
	registry, err := target.NewRegistry(debug.Translator{})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	_, ok := registry.Get("missing")
	if ok {
		t.Fatal(`Get("missing") ok = true`)
	}
}

func TestRegistryNames(t *testing.T) {
	registry, err := target.NewRegistry(xds.Translator{}, debug.Translator{})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	got := registry.Names()
	want := []string{"debug", "xds"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %#v, want %#v", got, want)
	}
}

func TestRegistryDuplicateTarget(t *testing.T) {
	_, err := target.NewRegistry(fakeTranslator{target: "debug"}, debug.Translator{})
	if err == nil {
		t.Fatal("NewRegistry() error = nil")
	}
	if !strings.Contains(err.Error(), `duplicate target "debug"`) {
		t.Fatalf("NewRegistry() error = %v", err)
	}
}

type fakeTranslator struct {
	target string
}

func (t fakeTranslator) Target() string {
	return t.target
}

func (t fakeTranslator) Translate(logical ir.LogicalGateway) (runtime.RuntimeSnapshot, error) {
	return runtime.RuntimeSnapshot{
		Target:  t.target,
		Gateway: logical.Name,
	}, nil
}

package builtin_test

import (
	"reflect"
	"testing"

	"github.com/lgc202/ingate/internal/core/target/builtin"
)

func TestNewRegistry(t *testing.T) {
	registry, err := builtin.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	got := registry.Names()
	want := []string{"debug", "xds"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %#v, want %#v", got, want)
	}
}

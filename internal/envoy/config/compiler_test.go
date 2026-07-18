package config

import (
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestCompilerCarriesResourceGenerationsWithoutChangingConfigVersion(t *testing.T) {
	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "gateway-1",
			UID:        types.UID("gateway-uid"),
			Generation: 1,
		},
	}
	first := (Compiler{}).Compile(ResourceSet{Gateways: []*gatewayv1.Gateway{gateway}})
	if first.HasErrors() {
		t.Fatalf("Compiler.Compile(generation 1) diagnostics = %v, want no errors", first.Diagnostics)
	}

	gateway.Generation = 2
	second := (Compiler{}).Compile(ResourceSet{Gateways: []*gatewayv1.Gateway{gateway}})
	if second.HasErrors() {
		t.Fatalf("Compiler.Compile(generation 2) diagnostics = %v, want no errors", second.Diagnostics)
	}
	if first.Version != second.Version {
		t.Errorf("Compiler.Compile() version changed from %q to %q for provenance-only change", first.Version, second.Version)
	}
	want := []ResourceGeneration{{
		Kind:       gatewayv1.KindGateway,
		Name:       gateway.Name,
		UID:        gateway.UID,
		Generation: gateway.Generation,
	}}
	if !slices.Equal(second.Resources, want) {
		t.Errorf("Compiler.Compile() resources = %v, want %v", second.Resources, want)
	}
}

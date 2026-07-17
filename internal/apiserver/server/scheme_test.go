package server

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/spec"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/pkg/generated/openapi"
)

func TestSchemeRegistersInternalGatewayTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind string
		want runtime.Object
	}{
		{kind: string(resource.KindGateway), want: &resource.Gateway{}},
		{kind: string(resource.KindRoute), want: &resource.Route{}},
		{kind: string(resource.KindUpstream), want: &resource.Upstream{}},
		{kind: string(resource.KindRateLimitPolicy), want: &resource.RateLimitPolicy{}},
		{kind: string(resource.KindAccessControlPolicy), want: &resource.AccessControlPolicy{}},
		{kind: string(resource.KindPolicyBinding), want: &resource.PolicyBinding{}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.kind, func(t *testing.T) {
			t.Parallel()

			gvk := schema.GroupVersionKind{
				Group:   resource.GroupName,
				Version: runtime.APIVersionInternal,
				Kind:    tt.kind,
			}

			obj, err := Scheme.New(gvk)
			if err != nil {
				t.Fatalf("Scheme.New(%s) failed: %v", gvk, err)
			}
			if got, want := reflect.TypeOf(obj), reflect.TypeOf(tt.want); got != want {
				t.Fatalf("created object = %T, want %T", got, want)
			}
		})
	}
}

func TestSchemeDoesNotRegisterRemovedGatewayTypes(t *testing.T) {
	t.Parallel()

	versions := []string{runtime.APIVersionInternal, gatewayv1.Version}
	kinds := []string{"RuntimeGroup", "RuntimeSnapshot", "RedisStore"}
	for _, version := range versions {
		for _, kind := range kinds {
			t.Run(version+"/"+kind, func(t *testing.T) {
				t.Parallel()

				gvk := schema.GroupVersionKind{
					Group:   resource.GroupName,
					Version: version,
					Kind:    kind,
				}
				if _, err := Scheme.New(gvk); !runtime.IsNotRegisteredError(err) {
					t.Errorf("Scheme.New(%s) error = %v, want not registered error", gvk, err)
				}
			})
		}
	}
}

func TestSchemeConvertsGatewayResourcesBetweenExternalAndInternalVersions(t *testing.T) {
	t.Parallel()

	tests := []runtime.Object{
		&gatewayv1.Gateway{},
		&gatewayv1.Route{},
		&gatewayv1.Upstream{},
		&gatewayv1.RateLimitPolicy{},
		&gatewayv1.AccessControlPolicy{},
		&gatewayv1.PolicyBinding{},
	}

	for _, external := range tests {
		external := external
		t.Run(reflect.TypeOf(external).Elem().Name(), func(t *testing.T) {
			t.Parallel()

			internalObj, err := Scheme.ConvertToVersion(external, resource.SchemeGroupVersion)
			if err != nil {
				t.Fatalf("ConvertToVersion(v1 -> internal) failed: %v", err)
			}
			externalObj, err := Scheme.ConvertToVersion(internalObj, gatewayv1.SchemeGroupVersion)
			if err != nil {
				t.Fatalf("ConvertToVersion(internal -> v1) failed: %v", err)
			}
			if got, want := reflect.TypeOf(externalObj), reflect.TypeOf(external); got != want {
				t.Fatalf("converted object = %T, want %T", got, want)
			}
		})
	}
}

func TestGeneratedOpenAPIDoesNotExposeRemovedGatewayModel(t *testing.T) {
	t.Parallel()

	definitions := openapi.GetOpenAPIDefinitions(func(path string) spec.Ref {
		return spec.MustCreateRef(path)
	})
	removedDefinitions := []string{
		"github.com/lgc202/ingate/pkg/apis/gateway/v1.RuntimeGroup",
		"github.com/lgc202/ingate/pkg/apis/gateway/v1.RuntimeSnapshot",
		"github.com/lgc202/ingate/pkg/apis/gateway/v1.RedisStore",
		"github.com/lgc202/ingate/pkg/apis/gateway/v1.RuntimeGroupRef",
		"github.com/lgc202/ingate/pkg/apis/gateway/v1.GlobalRateLimitConfig",
		"github.com/lgc202/ingate/pkg/apis/gateway/v1.Bundle",
	}
	for _, name := range removedDefinitions {
		if _, ok := definitions[name]; ok {
			t.Errorf("OpenAPI definition %q is still registered", name)
		}
	}

	gatewaySpecName := "github.com/lgc202/ingate/pkg/apis/gateway/v1.GatewaySpec"
	gatewaySpec, ok := definitions[gatewaySpecName]
	if !ok {
		t.Fatalf("OpenAPI definition %q is not registered", gatewaySpecName)
	}
	if _, ok := gatewaySpec.Schema.Properties["runtimeGroupRef"]; ok {
		t.Error("GatewaySpec OpenAPI still exposes runtimeGroupRef")
	}
	rateLimitPolicySpecName := "github.com/lgc202/ingate/pkg/apis/gateway/v1.RateLimitPolicySpec"
	rateLimitPolicySpec, ok := definitions[rateLimitPolicySpecName]
	if !ok {
		t.Fatalf("OpenAPI definition %q is not registered", rateLimitPolicySpecName)
	}
	if _, ok := rateLimitPolicySpec.Schema.Properties["global"]; ok {
		t.Error("RateLimitPolicySpec OpenAPI still exposes global")
	}
	if got, want := gatewayv1.RateLimitModeGlobal, gatewayv1.RateLimitMode("Global"); got != want {
		t.Errorf("RateLimitModeGlobal = %q, want %q", got, want)
	}
}

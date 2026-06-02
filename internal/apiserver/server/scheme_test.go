package server

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
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
		{kind: "RuntimeSnapshot", want: &resource.RuntimeSnapshot{}},
		{kind: string(resource.KindAIProvider), want: &resource.AIProvider{}},
		{kind: string(resource.KindAIModel), want: &resource.AIModel{}},
		{kind: string(resource.KindAIRoute), want: &resource.AIRoute{}},
		{kind: string(resource.KindAIPolicy), want: &resource.AIPolicy{}},
		{kind: string(resource.KindPlugin), want: &resource.Plugin{}},
		{kind: string(resource.KindPluginBinding), want: &resource.PluginBinding{}},
		{kind: string(resource.KindAuthPolicy), want: &resource.AuthPolicy{}},
		{kind: string(resource.KindRateLimitPolicy), want: &resource.RateLimitPolicy{}},
		{kind: "PolicyBinding", want: &resource.PolicyBinding{}},
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

func TestSchemeConvertsGatewayResourcesBetweenExternalAndInternalVersions(t *testing.T) {
	t.Parallel()

	tests := []runtime.Object{
		&gatewayv1.Gateway{},
		&gatewayv1.Route{},
		&gatewayv1.Upstream{},
		&gatewayv1.RuntimeSnapshot{},
		&gatewayv1.AIProvider{},
		&gatewayv1.AIModel{},
		&gatewayv1.AIRoute{},
		&gatewayv1.AIPolicy{},
		&gatewayv1.Plugin{},
		&gatewayv1.PluginBinding{},
		&gatewayv1.AuthPolicy{},
		&gatewayv1.RateLimitPolicy{},
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

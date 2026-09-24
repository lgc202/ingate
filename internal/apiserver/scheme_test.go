package apiserver

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lgc202/ingate/pkg/apis/gateway"
	"github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestNewScheme(t *testing.T) {
	scheme, err := newScheme()
	if err != nil {
		t.Fatalf("newScheme() error = %v, want nil", err)
	}

	for _, tt := range []struct {
		gvk      schema.GroupVersionKind
		wantType reflect.Type
	}{
		{gvk: gateway.SchemeGroupVersion.WithKind("Gateway"), wantType: reflect.TypeFor[*gateway.Gateway]()},
		{gvk: v1alpha1.SchemeGroupVersion.WithKind("Gateway"), wantType: reflect.TypeFor[*v1alpha1.Gateway]()},
		{gvk: metav1.SchemeGroupVersion.WithKind("Table"), wantType: reflect.TypeFor[*metav1.Table]()},
		{gvk: schema.GroupVersion{Version: "v1"}.WithKind("ListOptions"), wantType: reflect.TypeFor[*metav1.ListOptions]()},
	} {
		obj, err := scheme.New(tt.gvk)
		if err != nil {
			t.Errorf("Scheme.New(%s) error = %v, want nil", tt.gvk, err)
			continue
		}
		if got := reflect.TypeOf(obj); got != tt.wantType {
			t.Errorf("Scheme.New(%s) type = %v, want %v", tt.gvk, got, tt.wantType)
		}
	}
}

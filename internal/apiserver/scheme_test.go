package apiserver

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	coreapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/lgc202/ingate/pkg/apis/gateway"
	"github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestNewScheme(t *testing.T) {
	scheme, err := newScheme()
	if err != nil {
		t.Fatalf("newScheme() error = %v, want nil", err)
	}

	tests := []struct {
		gvk      schema.GroupVersionKind
		wantType reflect.Type
	}{
		{gvk: coreapi.SchemeGroupVersion.WithKind("Namespace"), wantType: reflect.TypeFor[*coreapi.Namespace]()},
		{gvk: coreapi.SchemeGroupVersion.WithKind("NamespaceList"), wantType: reflect.TypeFor[*coreapi.NamespaceList]()},
		{gvk: corev1.SchemeGroupVersion.WithKind("Namespace"), wantType: reflect.TypeFor[*corev1.Namespace]()},
		{gvk: corev1.SchemeGroupVersion.WithKind("NamespaceList"), wantType: reflect.TypeFor[*corev1.NamespaceList]()},
		{gvk: coreapi.SchemeGroupVersion.WithKind("ConfigMap"), wantType: reflect.TypeFor[*coreapi.ConfigMap]()},
		{gvk: coreapi.SchemeGroupVersion.WithKind("ConfigMapList"), wantType: reflect.TypeFor[*coreapi.ConfigMapList]()},
		{gvk: corev1.SchemeGroupVersion.WithKind("ConfigMap"), wantType: reflect.TypeFor[*corev1.ConfigMap]()},
		{gvk: corev1.SchemeGroupVersion.WithKind("ConfigMapList"), wantType: reflect.TypeFor[*corev1.ConfigMapList]()},
		{gvk: gateway.SchemeGroupVersion.WithKind("Gateway"), wantType: reflect.TypeFor[*gateway.Gateway]()},
		{gvk: v1alpha1.SchemeGroupVersion.WithKind("Gateway"), wantType: reflect.TypeFor[*v1alpha1.Gateway]()},
		{gvk: metav1.SchemeGroupVersion.WithKind("Table"), wantType: reflect.TypeFor[*metav1.Table]()},
		{gvk: schema.GroupVersion{Version: "v1"}.WithKind("ListOptions"), wantType: reflect.TypeFor[*metav1.ListOptions]()},
	}

	for _, tt := range tests {
		obj, err := scheme.New(tt.gvk)
		if err != nil {
			t.Errorf("Scheme.New(%s) error = %v, want nil", tt.gvk, err)
			continue
		}

		if got := reflect.TypeOf(obj); got != tt.wantType {
			t.Errorf("Scheme.New(%s) type = %v, want %v", tt.gvk, got, tt.wantType)
		}
	}

	for _, kind := range []string{"Pod", "Secret", "Service"} {
		if gvk := corev1.SchemeGroupVersion.WithKind(kind); !scheme.Recognizes(gvk) {
			t.Errorf("Scheme.Recognizes(%s) = false, want upstream core type registration", gvk)
		}
	}
}

func TestNewSchemeIsolation(t *testing.T) {
	first, err := newScheme()
	if err != nil {
		t.Fatalf("newScheme(first) error = %v", err)
	}

	second, err := newScheme()
	if err != nil {
		t.Fatalf("newScheme(second) error = %v", err)
	}

	if first == second || first == legacyscheme.Scheme || second == legacyscheme.Scheme {
		t.Error("newScheme() reused an instance, want separate private schemes")
	}

	gvk := v1alpha1.SchemeGroupVersion.WithKind("Gateway")
	if !first.Recognizes(gvk) || !second.Recognizes(gvk) {
		t.Errorf("private Scheme recognizes %s = %t/%t, want true/true", gvk, first.Recognizes(gvk), second.Recognizes(gvk))
	}

	if legacyscheme.Scheme.Recognizes(gvk) {
		t.Errorf("upstream global Scheme recognizes %s = true, want no business registration", gvk)
	}
}

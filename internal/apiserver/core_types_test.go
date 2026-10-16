package apiserver

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	core "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/utils/ptr"
)

func TestConfigMapCodecRoundTrip(t *testing.T) {
	scheme, err := newScheme()
	if err != nil {
		t.Fatalf("newScheme() error = %v", err)
	}

	original := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "team-a", Name: "runtime", ResourceVersion: "17",
			Labels: map[string]string{"owner": "platform"},
		},
		Immutable:  ptr.To(true),
		Data:       map[string]string{"config.json": "配置"},
		BinaryData: map[string][]byte{"bytes": {0, 255}},
	}
	internal := &core.ConfigMap{}
	if err := scheme.Convert(original, internal, nil); err != nil {
		t.Fatalf("Scheme.Convert(ConfigMap to internal) error = %v", err)
	}

	codec := serializer.NewCodecFactory(scheme).LegacyCodec(corev1.SchemeGroupVersion)
	data, err := runtime.Encode(codec, internal)
	if err != nil {
		t.Fatalf("Encode(internal ConfigMap) error = %v", err)
	}

	decoded, gvk, err := codec.Decode(data, nil, nil)
	if err != nil {
		t.Fatalf("Decode(ConfigMap) error = %v", err)
	}

	if want := corev1.SchemeGroupVersion.WithKind("ConfigMap"); gvk == nil || *gvk != want {
		t.Errorf("Decode(ConfigMap) GVK = %v, want %s", gvk, want)
	}

	roundTripped := &corev1.ConfigMap{}
	if err := scheme.Convert(decoded, roundTripped, nil); err != nil {
		t.Fatalf("Scheme.Convert(decoded ConfigMap to v1) error = %v", err)
	}

	if diff := cmp.Diff(original, roundTripped); diff != "" {
		t.Errorf("ConfigMap round trip mismatch (-want +got):\n%s", diff)
	}
}

func TestConfigMapListConversionAndDeepCopy(t *testing.T) {
	scheme, err := newScheme()
	if err != nil {
		t.Fatalf("newScheme() error = %v", err)
	}

	original := &corev1.ConfigMapList{
		ListMeta: metav1.ListMeta{ResourceVersion: "17", Continue: "next"},
		Items: []corev1.ConfigMap{{
			ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "runtime", Labels: map[string]string{"owner": "platform"}},
			Immutable:  ptr.To(true),
			Data:       map[string]string{"config.json": "{}"},
			BinaryData: map[string][]byte{"bytes": {0, 255}},
		}},
	}
	before := original.DeepCopy()
	internal := &core.ConfigMapList{}
	if err := scheme.Convert(original, internal, nil); err != nil {
		t.Fatalf("Scheme.Convert(ConfigMapList to internal) error = %v", err)
	}

	roundTripped := &corev1.ConfigMapList{}
	if err := scheme.Convert(internal, roundTripped, nil); err != nil {
		t.Fatalf("Scheme.Convert(ConfigMapList to v1) error = %v", err)
	}

	if diff := cmp.Diff(original, roundTripped); diff != "" {
		t.Errorf("ConfigMapList round trip mismatch (-want +got):\n%s", diff)
	}

	copied := internal.DeepCopy()
	copied.Items[0].Data["config.json"] = "other"
	copied.Items[0].BinaryData["bytes"][0] = 1
	*copied.Items[0].Immutable = false
	copied.Items[0].Labels["owner"] = "other"
	if got := internal.Items[0].Data["config.json"]; got != "{}" {
		t.Errorf("ConfigMapList.DeepCopy() changed original data = %q, want {}", got)
	}

	if got := internal.Items[0].BinaryData["bytes"][0]; got != 0 {
		t.Errorf("ConfigMapList.DeepCopy() changed original binary byte = %d, want 0", got)
	}

	if got := *internal.Items[0].Immutable; !got {
		t.Errorf("ConfigMapList.DeepCopy() changed original immutable = %t, want true", got)
	}

	if got := internal.Items[0].Labels["owner"]; got != "platform" {
		t.Errorf("ConfigMapList.DeepCopy() changed original label = %q, want platform", got)
	}

	if diff := cmp.Diff(before, original); diff != "" {
		t.Errorf("ConfigMapList.DeepCopy() shares mutable input (-want +got):\n%s", diff)
	}
}

func TestConfigMapEmptyDataDefaulting(t *testing.T) {
	scheme, err := newScheme()
	if err != nil {
		t.Fatalf("newScheme() error = %v", err)
	}

	for _, tt := range []struct {
		name string
		data map[string]string
		want map[string]string
	}{
		{name: "absent", want: map[string]string{}},
		{name: "present", data: map[string]string{"config.json": "{}"}, want: map[string]string{"config.json": "{}"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			configMap := &corev1.ConfigMap{Data: tt.data}
			scheme.Default(configMap)
			if diff := cmp.Diff(tt.want, configMap.Data); diff != "" {
				t.Errorf("Scheme.Default(ConfigMap %s) data mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

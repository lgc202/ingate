package install_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/lgc202/ingate/pkg/apis/core"
	"github.com/lgc202/ingate/pkg/apis/core/install"
)

func TestInstall(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := install.Install(scheme); err != nil {
		t.Fatalf("Install() error = %v, want nil", err)
	}

	for _, version := range []schema.GroupVersion{core.SchemeGroupVersion, corev1.SchemeGroupVersion} {
		for _, kind := range []string{"Namespace", "NamespaceList"} {
			gvk := version.WithKind(kind)
			if _, err := scheme.New(gvk); err != nil {
				t.Errorf("Scheme.New(%s) error = %v, want nil", gvk, err)
			}
		}
		for _, kind := range []string{"Pod", "Secret", "Service", "ConfigMap"} {
			if gvk := version.WithKind(kind); scheme.Recognizes(gvk) {
				t.Errorf("Scheme.Recognizes(%s) = true, want false", gvk)
			}
		}
	}
	if got := scheme.PrioritizedVersionsForGroup(""); !cmp.Equal(got, []schema.GroupVersion{corev1.SchemeGroupVersion}) {
		t.Errorf("Scheme.PrioritizedVersionsForGroup(empty) = %v, want [v1]", got)
	}
	if runtime.NewScheme().Recognizes(corev1.SchemeGroupVersion.WithKind("Namespace")) {
		t.Error("fresh Scheme recognizes Namespace, want instance-local registration")
	}
}

func TestNamespaceCodecRoundTrip(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := install.Install(scheme); err != nil {
		t.Fatalf("Install() error = %v, want nil", err)
	}
	codec := serializer.NewCodecFactory(scheme).LegacyCodec(corev1.SchemeGroupVersion)
	original := testNamespace()
	before := original.DeepCopy()
	internal := &core.Namespace{}
	if err := scheme.Convert(original, internal, nil); err != nil {
		t.Fatalf("Scheme.Convert(Namespace to internal) error = %v, want nil", err)
	}
	if diff := cmp.Diff(original.Spec, internal.Spec); diff != "" {
		t.Errorf("converted spec mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(original.Status, internal.Status); diff != "" {
		t.Errorf("converted status mismatch (-want +got):\n%s", diff)
	}

	data, err := runtime.Encode(codec, internal)
	if err != nil {
		t.Fatalf("Encode(internal Namespace) error = %v, want nil", err)
	}
	decoded, gvk, err := codec.Decode(data, nil, nil)
	if err != nil {
		t.Fatalf("Decode(Namespace) error = %v, want nil", err)
	}
	if want := corev1.SchemeGroupVersion.WithKind("Namespace"); gvk == nil || *gvk != want {
		t.Errorf("Decode(Namespace) GVK = %v, want %s", gvk, want)
	}
	roundTripped := &corev1.Namespace{}
	if err := scheme.Convert(decoded, roundTripped, nil); err != nil {
		t.Fatalf("Scheme.Convert(decoded Namespace to v1) error = %v, want nil", err)
	}
	if diff := cmp.Diff(original, roundTripped); diff != "" {
		t.Errorf("Namespace round trip mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(before, original); diff != "" {
		t.Errorf("Namespace input changed (-want +got):\n%s", diff)
	}
}

func TestNamespaceCodecDefaults(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := install.Install(scheme); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	codec := serializer.NewCodecFactory(scheme).LegacyCodec(corev1.SchemeGroupVersion)
	data := []byte(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"team-b"}}`)
	obj, _, err := codec.Decode(data, nil, nil)
	if err != nil {
		t.Fatalf("Decode(Namespace without defaults) error = %v", err)
	}
	namespace := obj.(*core.Namespace)
	if namespace.Status.Phase != corev1.NamespaceActive {
		t.Errorf("Decode() phase = %q, want Active", namespace.Status.Phase)
	}
	if namespace.Labels[corev1.LabelMetadataName] != namespace.Name {
		t.Errorf("Decode() labels = %v, want name label team-b", namespace.Labels)
	}
	if len(namespace.Spec.Finalizers) != 0 {
		t.Errorf("Decode() spec.finalizers = %v, want empty", namespace.Spec.Finalizers)
	}
}

func TestNamespaceListConversionAndDeepCopy(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := install.Install(scheme); err != nil {
		t.Fatalf("Install() error = %v, want nil", err)
	}
	original := &corev1.NamespaceList{
		ListMeta: metav1.ListMeta{ResourceVersion: "17", Continue: "next"},
		Items:    []corev1.Namespace{*testNamespace()},
	}
	internal := &core.NamespaceList{}
	if err := scheme.Convert(original, internal, nil); err != nil {
		t.Fatalf("Scheme.Convert(NamespaceList to internal) error = %v, want nil", err)
	}
	roundTripped := &corev1.NamespaceList{}
	if err := scheme.Convert(internal, roundTripped, nil); err != nil {
		t.Fatalf("Scheme.Convert(NamespaceList to v1) error = %v, want nil", err)
	}
	if diff := cmp.Diff(original, roundTripped); diff != "" {
		t.Errorf("NamespaceList round trip mismatch (-want +got):\n%s", diff)
	}

	before := original.DeepCopy()
	copied := internal.DeepCopy()
	copied.Items[0].Labels["owner"] = "other"
	copied.Items[0].Spec.Finalizers[0] = "example.com/other"
	copied.Items[0].Status.Conditions[0].Message = "other"
	if diff := cmp.Diff(before, original); diff != "" {
		t.Errorf("NamespaceList.DeepCopy() shares mutable fields (-want +got):\n%s", diff)
	}
}

func testNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "team-a",
			UID:             "namespace-uid",
			ResourceVersion: "17",
			Labels:          map[string]string{"owner": "platform", corev1.LabelMetadataName: "team-a"},
			Annotations:     map[string]string{"note": "namespace"},
			Finalizers:      []string{"example.com/metadata-cleanup"},
		},
		Spec: corev1.NamespaceSpec{Finalizers: []corev1.FinalizerName{corev1.FinalizerKubernetes, "example.com/cleanup"}},
		Status: corev1.NamespaceStatus{
			Phase: corev1.NamespaceActive,
			Conditions: []corev1.NamespaceCondition{{
				Type:               corev1.NamespaceDeletionDiscoveryFailure,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.NewTime(time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)),
				Reason:             "ResourcesDiscovered",
				Message:            "all resources discovered",
			}},
		},
	}
}

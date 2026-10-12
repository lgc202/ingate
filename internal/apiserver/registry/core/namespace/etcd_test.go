package namespace

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"

	coreapi "github.com/lgc202/ingate/pkg/apis/core"
)

type restOptionsGetterFunc func(schema.GroupResource, runtime.Object) (generic.RESTOptions, error)

func (f restOptionsGetterFunc) GetRESTOptions(
	resource schema.GroupResource,
	obj runtime.Object,
) (generic.RESTOptions, error) {
	return f(resource, obj)
}

func TestGetAttrs(t *testing.T) {
	namespace := &coreapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{"owner": "platform"}},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}
	gotLabels, gotFields, err := getAttrs(namespace)
	if err != nil {
		t.Fatalf("getAttrs(Namespace) error = %v", err)
	}
	if diff := cmp.Diff(labels.Set{"owner": "platform"}, gotLabels); diff != "" {
		t.Errorf("labels mismatch (-want +got):\n%s", diff)
	}
	wantFields := fields.Set{"metadata.name": "team-a", "status.phase": "Active"}
	if diff := cmp.Diff(wantFields, gotFields); diff != "" {
		t.Errorf("fields mismatch (-want +got):\n%s", diff)
	}
	if _, _, err := getAttrs(&coreapi.NamespaceList{}); err == nil {
		t.Error("getAttrs(NamespaceList) error = nil, want wrong type")
	}
}

func TestMatcher(t *testing.T) {
	namespace := &coreapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{"owner": "platform"}},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}
	tests := []struct {
		name          string
		labelSelector labels.Selector
		fieldSelector fields.Selector
		want          bool
	}{
		{
			name:          "matching",
			labelSelector: labels.SelectorFromSet(labels.Set{"owner": "platform"}),
			fieldSelector: fields.SelectorFromSet(fields.Set{"metadata.name": "team-a", "status.phase": "Active"}),
			want:          true,
		},
		{
			name:          "different_label",
			labelSelector: labels.SelectorFromSet(labels.Set{"owner": "other"}),
			fieldSelector: fields.Everything(),
		},
		{
			name:          "different_name",
			labelSelector: labels.Everything(),
			fieldSelector: fields.OneTermEqualSelector("metadata.name", "other"),
		},
		{
			name:          "different_phase",
			labelSelector: labels.Everything(),
			fieldSelector: fields.OneTermEqualSelector("status.phase", "Terminating"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicate := matcher(tt.labelSelector, tt.fieldSelector)
			got, err := predicate.Matches(namespace)
			if err != nil {
				t.Fatalf("Matches(Namespace) error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Matches(Namespace) = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestRESTExposesOnlyLifecycleOperations(t *testing.T) {
	if _, ok := any(&REST{}).(rest.CollectionDeleter); ok {
		t.Error("REST implements DeleteCollection, want lifecycle-only deletion")
	}
	for _, storage := range []any{&StatusREST{}, &FinalizeREST{}} {
		if _, ok := storage.(rest.Creater); ok {
			t.Errorf("%T implements Create, want false", storage)
		}
		if _, ok := storage.(rest.Lister); ok {
			t.Errorf("%T implements List, want false", storage)
		}
		if _, ok := storage.(rest.GracefulDeleter); ok {
			t.Errorf("%T implements Delete, want false", storage)
		}
		if _, ok := storage.(rest.Watcher); ok {
			t.Errorf("%T implements Watch, want false", storage)
		}
		if _, ok := storage.(rest.CollectionDeleter); ok {
			t.Errorf("%T implements DeleteCollection, want false", storage)
		}
	}
	if _, ok := any(&FinalizeREST{}).(rest.Getter); ok {
		t.Error("FinalizeREST implements Get/Patch, want update only")
	}
}

func TestNewRESTReturnsOptionsError(t *testing.T) {
	wantErr := errors.New("options unavailable")
	optionsGetter := restOptionsGetterFunc(func(
		resource schema.GroupResource,
		obj runtime.Object,
	) (generic.RESTOptions, error) {
		if want := coreapi.SchemeGroupVersion.WithResource("namespaces").GroupResource(); resource != want {
			t.Errorf("NewREST() resource = %v, want %v", resource, want)
		}
		if _, ok := obj.(*coreapi.Namespace); !ok {
			t.Errorf("NewREST() object = %T, want *core.Namespace", obj)
		}
		return generic.RESTOptions{}, wantErr
	})
	mainREST, statusREST, finalizeREST, err := NewREST(runtime.NewScheme(), optionsGetter)
	if !errors.Is(err, wantErr) {
		t.Errorf("NewREST() error = %v, want errors.Is(error, %v)", err, wantErr)
	}
	if mainREST != nil || statusREST != nil || finalizeREST != nil {
		t.Errorf("NewREST() storages = (%v, %v, %v), want nil", mainREST, statusREST, finalizeREST)
	}
}

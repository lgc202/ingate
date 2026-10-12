package modelroute

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"

	gatewayapi "github.com/lgc202/ingate/pkg/apis/gateway"
)

type restOptionsGetterFunc func(
	resource schema.GroupResource,
	obj runtime.Object,
) (generic.RESTOptions, error)

func TestGetAttrs(t *testing.T) {
	resource := &gatewayapi.ModelRoute{ObjectMeta: metav1.ObjectMeta{
		Name:      "edge",
		Namespace: "ignored",
		Labels:    map[string]string{"environment": "production"},
	}}

	gotLabels, gotFields, err := getAttrs(resource)
	if err != nil {
		t.Fatalf("getAttrs(%T) error = %v, want nil", resource, err)
	}

	if diff := cmp.Diff(labels.Set{"environment": "production"}, gotLabels); diff != "" {
		t.Errorf("getAttrs(%T) labels mismatch (-want +got):\n%s", resource, diff)
	}
	if diff := cmp.Diff(fields.Set{"metadata.name": "edge"}, gotFields); diff != "" {
		t.Errorf("getAttrs(%T) fields mismatch (-want +got):\n%s", resource, diff)
	}
}

func TestMatcher(t *testing.T) {
	resource := &gatewayapi.ModelRoute{ObjectMeta: metav1.ObjectMeta{
		Name:   "edge",
		Labels: map[string]string{"environment": "production"},
	}}

	tests := []struct {
		name          string
		labelSelector labels.Selector
		fieldSelector fields.Selector
		want          bool
	}{
		{
			name:          "matching labels and name",
			labelSelector: labels.SelectorFromSet(labels.Set{"environment": "production"}),
			fieldSelector: fields.OneTermEqualSelector("metadata.name", "edge"),
			want:          true,
		},
		{
			name:          "different label",
			labelSelector: labels.SelectorFromSet(labels.Set{"environment": "staging"}),
			fieldSelector: fields.Everything(),
		},
		{
			name:          "different name",
			labelSelector: labels.Everything(),
			fieldSelector: fields.OneTermEqualSelector("metadata.name", "other"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicate := matcher(tt.labelSelector, tt.fieldSelector)
			got, err := predicate.Matches(resource)
			if err != nil {
				t.Fatalf("matcher(%q, %q).Matches(%T) error = %v, want nil",
					tt.labelSelector, tt.fieldSelector, resource, err)
			}
			if got != tt.want {
				t.Errorf("matcher(%q, %q).Matches(%T) = %t, want %t",
					tt.labelSelector, tt.fieldSelector, resource, got, tt.want)
			}
		})
	}
}

func TestGetAttrsRejectsOtherResource(t *testing.T) {
	obj := &gatewayapi.Backend{}
	gotLabels, gotFields, err := getAttrs(obj)
	if err == nil || gotLabels != nil || gotFields != nil {
		t.Errorf("getAttrs(%T) = (%v, %v, %v), want nil attributes and an error", obj, gotLabels, gotFields, err)
	}
}

func TestNewRESTReturnsOptionsError(t *testing.T) {
	wantErr := errors.New("options unavailable")
	wantResource := gatewayapi.Resource("modelroutes")
	optionsGetter := restOptionsGetterFunc(func(
		resource schema.GroupResource,
		obj runtime.Object,
	) (generic.RESTOptions, error) {
		if resource != wantResource {
			t.Errorf("NewREST() resource = %v, want %v", resource, wantResource)
		}
		if _, ok := obj.(*gatewayapi.ModelRoute); !ok {
			t.Errorf("NewREST() object = %T, want *gateway.ModelRoute", obj)
		}
		return generic.RESTOptions{}, wantErr
	})

	mainREST, statusREST, err := NewREST(runtime.NewScheme(), optionsGetter)
	if !errors.Is(err, wantErr) {
		t.Errorf("NewREST() error = %v, want errors.Is(error, %v)", err, wantErr)
	}
	if mainREST != nil || statusREST != nil {
		t.Errorf("NewREST() = (%v, %v), want (nil, nil)", mainREST, statusREST)
	}
}

func (f restOptionsGetterFunc) GetRESTOptions(
	resource schema.GroupResource,
	obj runtime.Object,
) (generic.RESTOptions, error) {
	return f(resource, obj)
}

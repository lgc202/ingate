package backend

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
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apiserver/pkg/storage/storagebackend/factory"
	"k8s.io/client-go/tools/cache"

	gatewayapi "github.com/lgc202/ingate/pkg/apis/gateway"
)

// restOptionsSpy 只记录 Store 传给存储装饰器的属性函数，不提供 CRUD 或 Watch。
type restOptionsSpy struct {
	attrs storage.AttrFunc
	err   error
}

func (s *restOptionsSpy) GetRESTOptions(
	_ schema.GroupResource,
	_ runtime.Object,
) (generic.RESTOptions, error) {
	if s.err != nil {
		return generic.RESTOptions{}, s.err
	}

	return generic.RESTOptions{
		StorageConfig:  &storagebackend.ConfigForResource{},
		ResourcePrefix: "backends",
		Decorator:      s.decorate,
	}, nil
}

func (s *restOptionsSpy) decorate(
	_ *storagebackend.ConfigForResource,
	_ string,
	_ func(runtime.Object) (string, error),
	_ func() runtime.Object,
	_ func() runtime.Object,
	attrs storage.AttrFunc,
	_ storage.IndexerFuncs,
	_ *cache.Indexers,
) (storage.Interface, factory.DestroyFunc, error) {
	s.attrs = attrs
	// 本用例只检查装配，真实持久化和筛选由 API Server integration 覆盖。
	return nil, func() {}, nil
}

func TestRESTDefaultAttributes(t *testing.T) {
	mainREST, attrs := newRESTForTest(t)
	backend := &gatewayapi.Backend{ObjectMeta: metav1.ObjectMeta{
		Name:      "network",
		Namespace: "team-a",
		Labels:    map[string]string{"environment": "production"},
	}}
	// Store 根据命名空间级 Strategy 选择原生默认属性函数。
	predicate := mainREST.PredicateFunc(labels.Everything(), fields.Everything())

	for _, tt := range []struct {
		name  string
		attrs storage.AttrFunc
	}{
		{name: "storage decorator", attrs: attrs},
		{name: "selection predicate", attrs: predicate.GetAttrs},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.attrs == nil {
				t.Fatal("NewREST() attributes = nil, want completed default function")
			}
			gotLabels, gotFields, err := tt.attrs(backend)
			if err != nil {
				t.Fatalf("attributes(%T) error = %v, want nil", backend, err)
			}
			if diff := cmp.Diff(labels.Set{"environment": "production"}, gotLabels); diff != "" {
				t.Errorf("attributes(%T) labels mismatch (-want +got):\n%s", backend, diff)
			}
			if diff := cmp.Diff(fields.Set{"metadata.name": "network", "metadata.namespace": "team-a"}, gotFields); diff != "" {
				t.Errorf("attributes(%T) fields mismatch (-want +got):\n%s", backend, diff)
			}
			if _, _, err := tt.attrs(&runtime.Unknown{}); err == nil {
				t.Error("attributes(*runtime.Unknown) error = nil, want missing metadata error")
			}
		})
	}
}

func TestRESTDefaultSelection(t *testing.T) {
	mainREST, _ := newRESTForTest(t)
	backend := &gatewayapi.Backend{ObjectMeta: metav1.ObjectMeta{
		Namespace: "team-a",
		Name:      "network",
		Labels:    map[string]string{"environment": "production"},
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
			fieldSelector: fields.OneTermEqualSelector("metadata.name", "network"),
			want:          true,
		},
		{
			name:          "matching namespace",
			labelSelector: labels.Everything(),
			fieldSelector: fields.OneTermEqualSelector("metadata.namespace", "team-a"),
			want:          true,
		},
		{
			name:          "different namespace",
			labelSelector: labels.Everything(),
			fieldSelector: fields.OneTermEqualSelector("metadata.namespace", "team-b"),
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
			predicate := mainREST.PredicateFunc(tt.labelSelector, tt.fieldSelector)
			got, err := predicate.Matches(backend)
			if err != nil {
				t.Fatalf("PredicateFunc(%q, %q).Matches(%q) error = %v, want nil",
					tt.labelSelector, tt.fieldSelector, backend.Name, err)
			}
			if got != tt.want {
				t.Errorf("PredicateFunc(%q, %q).Matches(%q) = %t, want %t",
					tt.labelSelector, tt.fieldSelector, backend.Name, got, tt.want)
			}
		})
	}
}

func TestNewRESTReturnsOptionsError(t *testing.T) {
	wantErr := errors.New("options unavailable")
	spy := &restOptionsSpy{err: wantErr}

	mainREST, statusREST, err := NewREST(runtime.NewScheme(), spy)
	if !errors.Is(err, wantErr) {
		t.Errorf("NewREST() error = %v, want errors.Is(error, %v)", err, wantErr)
	}
	if mainREST != nil || statusREST != nil {
		t.Errorf("NewREST() = (%v, %v), want (nil, nil)", mainREST, statusREST)
	}
}

func newRESTForTest(t *testing.T) (*REST, storage.AttrFunc) {
	t.Helper()

	spy := &restOptionsSpy{}
	mainREST, _, err := NewREST(runtime.NewScheme(), spy)
	if err != nil {
		t.Fatalf("NewREST() error = %v, want nil", err)
	}
	t.Cleanup(mainREST.Destroy)
	if mainREST.PredicateFunc == nil || spy.attrs == nil {
		t.Fatal("NewREST() predicate/attributes = nil, want completed defaults")
	}

	return mainREST, spy.attrs
}

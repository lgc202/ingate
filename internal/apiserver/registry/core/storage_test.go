package core

import (
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apiserver/pkg/storage/storagebackend/factory"
	"k8s.io/client-go/tools/cache"
)

type restOptionsGetterFunc func(schema.GroupResource, runtime.Object) (generic.RESTOptions, error)

func (f restOptionsGetterFunc) GetRESTOptions(resource schema.GroupResource, example runtime.Object) (generic.RESTOptions, error) {
	return f(resource, example)
}

func TestNewStorageFailureCleanup(t *testing.T) {
	failure := errors.New("storage setup failed")
	for _, tt := range []struct {
		resource     string
		wantDestroys int
	}{
		{resource: "namespaces"},
		{resource: "configmaps", wantDestroys: 1},
	} {
		t.Run(tt.resource, func(t *testing.T) {
			destroys := 0
			getter := restOptionsGetterFunc(func(resource schema.GroupResource, _ runtime.Object) (generic.RESTOptions, error) {
				if resource.Resource == tt.resource {
					return generic.RESTOptions{}, failure
				}

				return generic.RESTOptions{
					ResourcePrefix: resource.Resource,
					StorageConfig:  &storagebackend.ConfigForResource{},
					Decorator: func(
						_ *storagebackend.ConfigForResource,
						_ string,
						_ func(runtime.Object) (string, error),
						_, _ func() runtime.Object,
						_ storage.AttrFunc,
						_ storage.IndexerFuncs,
						_ *cache.Indexers,
					) (storage.Interface, factory.DestroyFunc, error) {
						// 本用例只验证装配回滚，不操作底层存储；真实读写由 API integration 覆盖。
						return nil, func() { destroys++ }, nil
					},
				}, nil
			})

			result, err := NewStorage(getter)
			if !errors.Is(err, failure) {
				t.Errorf("NewStorage(%s failure) error = %v, want wrapped setup error", tt.resource, err)
			}

			if result != nil {
				t.Errorf("NewStorage(%s failure) storage = %v, want no partial map", tt.resource, result)
			}

			if destroys != tt.wantDestroys {
				t.Errorf("NewStorage(%s failure) Destroy calls = %d, want %d", tt.resource, destroys, tt.wantDestroys)
			}
		})
	}
}

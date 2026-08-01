// Package registry 提供 Ingate 资源接入 generic-apiserver 所需的共享 REST storage
package registry

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"
)

// StorageDefinition 描述一个带 status 子资源的 Ingate 声明式资源存储
type StorageDefinition struct {
	NewObject        func() runtime.Object
	NewList          func() runtime.Object
	Resource         schema.GroupResource
	SingularResource schema.GroupResource
	Strategy         rest.CreateUpdateResetFieldsStrategy
	StatusStrategy   rest.UpdateResetFieldsStrategy
}

// REST 实现声明式资源的 generic-apiserver RESTStorage
type REST struct {
	*genericregistry.Store
}

// StatusREST 实现声明式资源的 status 子资源存储
type StatusREST struct {
	store *genericregistry.Store
}

// NewStorage 创建共享同一个底层 store 的主资源与 status 子资源存储
func NewStorage(
	optsGetter generic.RESTOptionsGetter,
	definition StorageDefinition,
) (*REST, *StatusREST, error) {
	store := &genericregistry.Store{
		NewFunc:                   definition.NewObject,
		NewListFunc:               definition.NewList,
		DefaultQualifiedResource:  definition.Resource,
		SingularQualifiedResource: definition.SingularResource,
		CreateStrategy:            definition.Strategy,
		UpdateStrategy:            definition.Strategy,
		DeleteStrategy:            definition.Strategy,
		ResetFieldsStrategy:       definition.Strategy,
		TableConvertor:            rest.NewDefaultTableConvertor(definition.Resource),
	}
	if err := store.CompleteWithOptions(&generic.StoreOptions{RESTOptions: optsGetter}); err != nil {
		return nil, nil, err
	}

	statusStore := *store
	statusStore.UpdateStrategy = definition.StatusStrategy
	statusStore.ResetFieldsStrategy = definition.StatusStrategy
	return &REST{Store: store}, &StatusREST{store: &statusStore}, nil
}

// New 创建对应的资源对象
func (r *StatusREST) New() runtime.Object {
	return r.store.New()
}

// Destroy 不重复销毁主资源与 status 子资源共享的底层存储
func (r *StatusREST) Destroy() {
}

// Get 读取旧对象，供 status Patch 合并使用
func (r *StatusREST) Get(
	ctx context.Context,
	name string,
	options *metav1.GetOptions,
) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update 只通过 status strategy 更新子资源，不允许通过 PUT 创建对象
func (r *StatusREST) Update(
	ctx context.Context,
	name string,
	objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc,
	updateValidation rest.ValidateObjectUpdateFunc,
	_ bool,
	options *metav1.UpdateOptions,
) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, false, options)
}

// GetResetFields 返回 status strategy 会重置的字段
func (r *StatusREST) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return r.store.GetResetFields()
}

// ConvertToTable 使用主资源相同的默认表格转换规则
func (r *StatusREST) ConvertToTable(
	ctx context.Context,
	object runtime.Object,
	tableOptions runtime.Object,
) (*metav1.Table, error) {
	return r.store.ConvertToTable(ctx, object, tableOptions)
}

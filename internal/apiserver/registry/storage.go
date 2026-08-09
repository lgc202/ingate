// Package registry 提供 Ingate 资源接入 generic-apiserver 所需的共享 REST storage
package registry

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
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
	DisplayName      func(runtime.Object) string
}

// REST 实现声明式资源的 generic-apiserver RESTStorage
type REST struct {
	*genericregistry.Store
	displayName func(runtime.Object) string
	guard       *DisplayNameGuard
	resource    schema.GroupResource
}

// StatusREST 实现声明式资源的 status 子资源存储
type StatusREST struct {
	store *genericregistry.Store
}

// NewStorage 创建共享同一个底层 store 的主资源与 status 子资源存储
func NewStorage(
	optsGetter generic.RESTOptionsGetter,
	definition StorageDefinition,
	guard *DisplayNameGuard,
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
	return &REST{
		Store:       store,
		displayName: definition.DisplayName,
		guard:       guard,
		resource:    definition.Resource,
	}, &StatusREST{store: &statusStore}, nil
}

// Create 在写入前对同类资源的 displayName 做跨实例唯一性裁决
func (r *REST) Create(
	ctx context.Context,
	obj runtime.Object,
	createValidation rest.ValidateObjectFunc,
	options *metav1.CreateOptions,
) (runtime.Object, error) {
	var result runtime.Object
	err := r.guard.lock(ctx, r.resource, func() error {
		if err := r.validateDisplayName(ctx, obj); err != nil {
			return err
		}
		var err error
		result, err = r.Store.Create(ctx, obj, createValidation, options)
		return err
	})
	return result, err
}

// Update 让 PUT、Patch 和 Server-Side Apply 共用相同的 displayName 唯一性边界
func (r *REST) Update(
	ctx context.Context,
	name string,
	objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc,
	updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool,
	options *metav1.UpdateOptions,
) (runtime.Object, bool, error) {
	var result runtime.Object
	var created bool
	err := r.guard.lock(ctx, r.resource, func() error {
		var err error
		result, created, err = r.Store.Update(
			ctx,
			name,
			displayNameUpdatedObjectInfo{UpdatedObjectInfo: objInfo, validate: r.validateDisplayName},
			createValidation,
			updateValidation,
			forceAllowCreate,
			options,
		)
		return err
	})
	return result, created, err
}

func (r *REST) validateDisplayName(ctx context.Context, obj runtime.Object) error {
	displayName := r.displayName(obj)
	if displayName == "" {
		return nil
	}
	objectMeta, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	list, err := r.Store.List(ctx, &metainternalversion.ListOptions{})
	if err != nil {
		return err
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		return err
	}
	for _, current := range items {
		currentMeta, err := meta.Accessor(current)
		if err != nil {
			return err
		}
		if currentMeta.GetName() != objectMeta.GetName() && r.displayName(current) == displayName {
			return apierrors.NewAlreadyExists(r.resource, displayName)
		}
	}
	return nil
}

type displayNameUpdatedObjectInfo struct {
	rest.UpdatedObjectInfo
	validate func(context.Context, runtime.Object) error
}

func (i displayNameUpdatedObjectInfo) UpdatedObject(
	ctx context.Context,
	old runtime.Object,
) (runtime.Object, error) {
	obj, err := i.UpdatedObjectInfo.UpdatedObject(ctx, old)
	if err != nil {
		return nil, err
	}
	if err := i.validate(ctx, obj); err != nil {
		return nil, err
	}
	return obj, nil
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

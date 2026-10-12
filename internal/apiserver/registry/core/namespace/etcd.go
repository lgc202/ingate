/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// 来源：Kubernetes v1.37.0 pkg/registry/core/namespace/storage/storage.go。
// 本地适配：注入私有 Scheme，复用公开 TableConvertor；不嵌入完整 Store。
package namespace

import (
	"context"
	"fmt"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	coreapi "github.com/lgc202/ingate/pkg/apis/core"
)

// REST 提供 Namespace 存储，不暴露绕过生命周期的 DeleteCollection。
type REST struct {
	store *genericregistry.Store
}

// StatusREST 提供 Namespace status 的读取、更新和 Patch。
type StatusREST struct {
	store *genericregistry.Store
}

// FinalizeREST 仅提供 Namespace spec.finalizers 的更新入口。
type FinalizeREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Scoper                 = &REST{}
	_ rest.Lister                 = &REST{}
	_ rest.Creater                = &REST{}
	_ rest.Patcher                = &REST{}
	_ rest.GracefulDeleter        = &REST{}
	_ rest.Watcher                = &REST{}
	_ rest.TableConvertor         = &REST{}
	_ rest.SingularNameProvider   = &REST{}
	_ rest.ShortNamesProvider     = &REST{}
	_ rest.StorageVersionProvider = &REST{}
	_ rest.ResetFieldsStrategy    = &REST{}
	_ rest.Patcher                = &StatusREST{}
	_ rest.ResetFieldsStrategy    = &StatusREST{}
	_ rest.TableConvertor         = &StatusREST{}
	_ rest.Updater                = &FinalizeREST{}
	_ rest.ResetFieldsStrategy    = &FinalizeREST{}
)

// NewREST 创建 Namespace 主资源及 status、finalize 子资源的共享存储。
func NewREST(
	scheme *runtime.Scheme,
	optionsGetter generic.RESTOptionsGetter,
) (*REST, *StatusREST, *FinalizeREST, error) {
	strategy := NewStrategy(scheme)
	resource := coreapi.SchemeGroupVersion.WithResource("namespaces").GroupResource()
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &coreapi.Namespace{} },
		NewListFunc:               func() runtime.Object { return &coreapi.NamespaceList{} },
		PredicateFunc:             matcher,
		DefaultQualifiedResource:  resource,
		SingularQualifiedResource: coreapi.SchemeGroupVersion.WithResource("namespace").GroupResource(),
		CreateStrategy:            strategy,
		UpdateStrategy:            strategy,
		DeleteStrategy:            strategy,
		ResetFieldsStrategy:       strategy,
		ReturnDeletedObject:       true,
		ShouldDeleteDuringUpdate:  shouldDeleteDuringUpdate,
		TableConvertor:            rest.NewDefaultTableConvertor(resource),
	}
	options := &generic.StoreOptions{RESTOptions: optionsGetter, AttrFunc: getAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, nil, nil, fmt.Errorf("complete Namespace REST storage: %w", err)
	}

	statusStrategy := NewStatusStrategy(scheme)
	statusStore := *store
	statusStore.UpdateStrategy = statusStrategy
	statusStore.ResetFieldsStrategy = statusStrategy

	finalizeStrategy := NewFinalizeStrategy(scheme)
	finalizeStore := *store
	finalizeStore.UpdateStrategy = finalizeStrategy
	finalizeStore.ResetFieldsStrategy = finalizeStrategy

	return &REST{store: store}, &StatusREST{store: &statusStore}, &FinalizeREST{store: &finalizeStore}, nil
}

// NamespaceScoped 表示 Namespace 本身是集群级资源。
func (r *REST) NamespaceScoped() bool {
	return r.store.NamespaceScoped()
}

// GetSingularName 返回发现接口使用的资源单数名。
func (r *REST) GetSingularName() string {
	return r.store.GetSingularName()
}

// New 返回用于解码 Namespace 请求的空对象。
func (r *REST) New() runtime.Object {
	return r.store.New()
}

// Destroy 仅由主资源销毁共享存储。
func (r *REST) Destroy() {
	r.store.Destroy()
}

// NewList 返回 Namespace 列表对象。
func (r *REST) NewList() runtime.Object {
	return r.store.NewList()
}

// List 按标签和标准字段选择器列出 Namespace。
func (r *REST) List(
	ctx context.Context,
	options *metainternalversion.ListOptions,
) (runtime.Object, error) {
	return r.store.List(ctx, options)
}

// Create 通过标准策略创建 Namespace。
func (r *REST) Create(
	ctx context.Context,
	obj runtime.Object,
	createValidation rest.ValidateObjectFunc,
	options *metav1.CreateOptions,
) (runtime.Object, error) {
	return r.store.Create(ctx, obj, createValidation, options)
}

// Update 更新 Namespace 主资源，保留 status 和 spec.finalizers。
func (r *REST) Update(
	ctx context.Context,
	name string,
	objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc,
	updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool,
	options *metav1.UpdateOptions,
) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}

// Get 读取 Namespace，供主资源读取及 Patch 使用。
func (r *REST) Get(
	ctx context.Context,
	name string,
	options *metav1.GetOptions,
) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Watch 观察符合选择器的 Namespace 变化。
func (r *REST) Watch(
	ctx context.Context,
	options *metainternalversion.ListOptions,
) (watch.Interface, error) {
	return r.store.Watch(ctx, options)
}

// ConvertToTable 使用公开的默认表格转换器。
func (r *REST) ConvertToTable(
	ctx context.Context,
	obj runtime.Object,
	options runtime.Object,
) (*metav1.Table, error) {
	return r.store.ConvertToTable(ctx, obj, options)
}

// ShortNames 返回标准 Namespace 缩写。
func (*REST) ShortNames() []string {
	return []string{"ns"}
}

// StorageVersion 返回底层 Store 的存储编码版本。
func (r *REST) StorageVersion() runtime.GroupVersioner {
	return r.store.StorageVersion()
}

// GetResetFields 返回主资源更新时恢复的字段。
func (r *REST) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return r.store.GetResetFields()
}

// New 返回用于解码 status 更新的空 Namespace。
func (r *StatusREST) New() runtime.Object {
	return r.store.New()
}

// Destroy 不重复销毁与主资源共享的存储。
func (*StatusREST) Destroy() {}

// Get 读取 Namespace，供 status 读取及 Patch 使用。
func (r *StatusREST) Get(
	ctx context.Context,
	name string,
	options *metav1.GetOptions,
) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update 更新 status，不允许通过子资源创建对象。
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

// GetResetFields 返回 status 更新时恢复的字段。
func (r *StatusREST) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return r.store.GetResetFields()
}

// ConvertToTable 将 status 响应转换为标准表格。
func (r *StatusREST) ConvertToTable(
	ctx context.Context,
	obj runtime.Object,
	options runtime.Object,
) (*metav1.Table, error) {
	return r.store.ConvertToTable(ctx, obj, options)
}

// New 返回用于解码 finalize 更新的空 Namespace。
func (r *FinalizeREST) New() runtime.Object {
	return r.store.New()
}

// Destroy 不重复销毁与主资源共享的存储。
func (*FinalizeREST) Destroy() {}

// Update 更新 spec.finalizers，不允许通过子资源创建对象。
func (r *FinalizeREST) Update(
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

// GetResetFields 返回 finalize 更新时恢复的字段。
func (r *FinalizeREST) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return r.store.GetResetFields()
}

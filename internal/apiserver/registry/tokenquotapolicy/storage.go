// Package tokenquotapolicy 提供 TokenQuotaPolicy 的 apiserver 存储与校验
package tokenquotapolicy

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

// REST 实现 TokenQuotaPolicy 资源的 apiserver RESTStorage
type REST struct {
	*genericregistry.Store
}

// StatusREST 实现 TokenQuotaPolicy status 子资源存储
type StatusREST struct {
	store *genericregistry.Store
}

// NewREST 创建 TokenQuotaPolicy 资源存储
func NewREST(optsGetter generic.RESTOptionsGetter, typer runtime.ObjectTyper) (*REST, *StatusREST, error) {
	strategy := newStrategy(typer)
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &resource.TokenQuotaPolicy{} },
		NewListFunc:               func() runtime.Object { return &resource.TokenQuotaPolicyList{} },
		DefaultQualifiedResource:  resource.Resource(resource.ResourceTokenQuotaPolicies),
		SingularQualifiedResource: resource.Resource(resource.ResourceTokenQuotaPolicy),

		CreateStrategy:      strategy,
		UpdateStrategy:      strategy,
		DeleteStrategy:      strategy,
		ResetFieldsStrategy: strategy,

		TableConvertor: rest.NewDefaultTableConvertor(resource.Resource(resource.ResourceTokenQuotaPolicies)),
	}
	if err := store.CompleteWithOptions(&generic.StoreOptions{RESTOptions: optsGetter}); err != nil {
		return nil, nil, err
	}

	statusStore := *store
	statusStrategy := newStatusStrategy(typer)
	statusStore.UpdateStrategy = statusStrategy
	statusStore.ResetFieldsStrategy = statusStrategy

	return &REST{Store: store}, &StatusREST{store: &statusStore}, nil
}

// New 创建 TokenQuotaPolicy 对象
func (r *StatusREST) New() runtime.Object {
	return &resource.TokenQuotaPolicy{}
}

// Destroy 清理底层资源
func (r *StatusREST) Destroy() {
}

// Get 获取 TokenQuotaPolicy 对象，Patch status 时需要读取旧对象
func (r *StatusREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update 更新 TokenQuotaPolicy status 子资源
func (r *StatusREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, false, options)
}

// GetResetFields 返回 status 子资源会重置的字段
func (r *StatusREST) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return r.store.GetResetFields()
}

// ConvertToTable 转换 TokenQuotaPolicy 表格输出
func (r *StatusREST) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return r.store.ConvertToTable(ctx, object, tableOptions)
}

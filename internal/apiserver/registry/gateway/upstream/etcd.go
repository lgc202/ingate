package upstream

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	gatewayapi "github.com/lgc202/ingate/pkg/apis/gateway"
)

var (
	_ rest.StandardStorage     = &REST{}
	_ rest.Patcher             = &StatusREST{}
	_ rest.ResetFieldsStrategy = &StatusREST{}
	_ rest.TableConvertor      = &StatusREST{}
)

// REST 提供 Upstream 主资源的标准存储操作。
type REST struct {
	*genericregistry.Store
}

// NewREST 创建 Upstream 主资源及 Status 子资源的存储入口。
func NewREST(
	scheme *runtime.Scheme,
	optionsGetter generic.RESTOptionsGetter,
) (*REST, *StatusREST, error) {
	strategy := NewStrategy(scheme)
	store := &genericregistry.Store{
		NewFunc: func() runtime.Object {
			return &gatewayapi.Upstream{}
		},
		NewListFunc: func() runtime.Object {
			return &gatewayapi.UpstreamList{}
		},
		PredicateFunc:             matcher,
		DefaultQualifiedResource:  gatewayapi.Resource("upstreams"),
		SingularQualifiedResource: gatewayapi.Resource("upstream"),
		CreateStrategy:            strategy,
		UpdateStrategy:            strategy,
		DeleteStrategy:            strategy,
		ResetFieldsStrategy:       strategy,
		TableConvertor:            rest.NewDefaultTableConvertor(gatewayapi.Resource("upstreams")),
	}

	options := &generic.StoreOptions{
		RESTOptions: optionsGetter,
		AttrFunc:    getAttrs,
	}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, nil, fmt.Errorf("complete Upstream REST storage: %w", err)
	}

	statusStrategy := NewStatusStrategy(scheme)
	statusStore := *store
	statusStore.UpdateStrategy = statusStrategy
	statusStore.ResetFieldsStrategy = statusStrategy

	return &REST{Store: store}, &StatusREST{store: &statusStore}, nil
}

// StatusREST 仅提供 Upstream Status 的读取、更新和 Patch。
type StatusREST struct {
	store *genericregistry.Store
}

// New 返回用于解码 Status 更新请求的空 Upstream。
func (*StatusREST) New() runtime.Object {
	return &gatewayapi.Upstream{}
}

// Destroy 不重复销毁与主资源共享的底层 Store。
func (*StatusREST) Destroy() {}

// Get 读取 Upstream，供 Status 读取及 Patch 使用。
func (r *StatusREST) Get(
	ctx context.Context,
	name string,
	options *metav1.GetOptions,
) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update 更新 Upstream Status，且不允许经由子资源创建对象。
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

// GetResetFields 返回 Status 更新时由服务端恢复的字段。
func (r *StatusREST) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return r.store.GetResetFields()
}

// ConvertToTable 将 Upstream 转换为表格响应。
func (r *StatusREST) ConvertToTable(
	ctx context.Context,
	obj runtime.Object,
	options runtime.Object,
) (*metav1.Table, error) {
	return r.store.ConvertToTable(ctx, obj, options)
}

func getAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	upstream, ok := obj.(*gatewayapi.Upstream)
	if !ok {
		return nil, nil, fmt.Errorf("expected *gateway.Upstream, got %T", obj)
	}

	return labels.Set(upstream.Labels), generic.ObjectMetaFieldsSet(&upstream.ObjectMeta, false), nil
}

func matcher(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: getAttrs,
	}
}

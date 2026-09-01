package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/biz"
)

// createResourceClient 是通用资源存储需要的 typed client 能力。
type createResourceClient[T any] interface {
	resourceClient[T]
	Create(ctx context.Context, object T, options metav1.CreateOptions) (T, error)
}

// resourceStore 集中声明式资源 Store 共享的 CRUD 协议。
type resourceStore[Item any, Object resourceObject[Object], Spec any] struct {
	kind      string
	listKind  string
	client    func() createResourceClient[Object]
	list      func(context.Context, metav1.ListOptions) ([]Item, string, error)
	newObject func(string, Spec) Object
	setSpec   func(Object, Spec)
}

// ListPage 分页返回声明式资源。
func (s *resourceStore[Item, Object, Spec]) ListPage(
	ctx context.Context,
	page biz.PageRequest,
) (biz.PageResult[Item], error) {
	items, nextCursor, err := s.list(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[Item]{}, listError(s.listKind, err)
	}
	return biz.PageResult[Item]{Items: items, NextCursor: nextCursor}, nil
}

// Get 返回指定声明式资源。
func (s *resourceStore[Item, Object, Spec]) Get(
	ctx context.Context,
	resourceID string,
) (Object, error) {
	object, err := s.client().Get(ctx, resourceID, metav1.GetOptions{})
	return object, resourceError("get", s.kind, resourceID, err)
}

// ListByIDs 返回当前存在的指定声明式资源。
func (s *resourceStore[Item, Object, Spec]) ListByIDs(
	ctx context.Context,
	resourceIDs []string,
) (map[string]Object, error) {
	return listByIDs(ctx, resourceIDs, s.Get)
}

// Create 创建声明式资源。
func (s *resourceStore[Item, Object, Spec]) Create(
	ctx context.Context,
	resourceID string,
	spec Spec,
) (Object, error) {
	created, err := s.client().Create(
		ctx,
		s.newObject(resourceID, spec),
		metav1.CreateOptions{},
	)
	return created, resourceError("create", s.kind, resourceID, err)
}

// ReplaceSpec 完整替换声明式资源配置。
func (s *resourceStore[Item, Object, Spec]) ReplaceSpec(
	ctx context.Context,
	observed Object,
	spec Spec,
) (Object, error) {
	client := s.client()
	return replaceResourceSpec(
		ctx,
		client,
		s.kind,
		observed,
		func(object Object) { s.setSpec(object, spec) },
	)
}

// Delete 删除声明式资源。
func (s *resourceStore[Item, Object, Spec]) Delete(
	ctx context.Context,
	observed Object,
) error {
	return deleteResource(ctx, s.client(), s.kind, observed)
}

func newResourceStore[Item any, Object resourceObject[Object], Spec any](
	kind string,
	listKind string,
	client func() createResourceClient[Object],
	list func(context.Context, metav1.ListOptions) ([]Item, string, error),
	newObject func(string, Spec) Object,
	setSpec func(Object, Spec),
) *resourceStore[Item, Object, Spec] {
	return &resourceStore[Item, Object, Spec]{
		kind:      kind,
		listKind:  listKind,
		client:    client,
		list:      list,
		newObject: newObject,
		setSpec:   setSpec,
	}
}

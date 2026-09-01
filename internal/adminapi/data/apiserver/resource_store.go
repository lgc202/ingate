package apiserver

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
)

// 单个管理请求最多占用八条并发读取，避免大 Route 耗尽 API Server 连接。
const maxConcurrentResourceReads = 8

// resourceObject 是支持元数据访问和强类型深拷贝的声明式资源。
type resourceObject[T any] interface {
	metav1.Object
	DeepCopy() T
}

// resourceClient 是乐观并发写入所需的最小 Kubernetes typed client 能力。
type resourceClient[T any] interface {
	Get(ctx context.Context, name string, options metav1.GetOptions) (T, error)
	Update(ctx context.Context, object T, options metav1.UpdateOptions) (T, error)
	Delete(ctx context.Context, name string, options metav1.DeleteOptions) error
}

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

func listOptions(page biz.PageRequest) metav1.ListOptions {
	return metav1.ListOptions{Limit: page.Limit, Continue: page.Cursor}
}

func listError(kind string, err error) error {
	if apierrors.IsBadRequest(err) || apierrors.IsResourceExpired(err) {
		return fmt.Errorf("%w: %w", biz.ErrInvalidCursor, err)
	}
	return resourceError("list", kind, "", err)
}

func resourceError(operation, kind, name string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case apierrors.IsNotFound(err):
		err = fmt.Errorf("%w: %w", biz.ErrResourceNotFound, err)
	case apierrors.IsAlreadyExists(err):
		err = fmt.Errorf("%w: %w", biz.ErrResourceAlreadyExists, err)
	case apierrors.IsConflict(err):
		err = fmt.Errorf("%w: %w", biz.ErrResourceVersionConflict, err)
	case apierrors.IsInvalid(err):
		err = biz.NewInvalidResource(err)
	}
	if name == "" {
		return fmt.Errorf("%s %s: %w", operation, kind, err)
	}
	return fmt.Errorf("%s %s %q: %w", operation, kind, name, err)
}

// replaceResourceSpec 在资源身份和配置版本未变化时重试底层资源版本冲突。
func replaceResourceSpec[T resourceObject[T]](
	ctx context.Context,
	client resourceClient[T],
	kind string,
	observed T,
	setSpec func(T),
) (T, error) {
	name := observed.GetName()
	current := observed.DeepCopy()
	refreshRequired := false
	var updated T
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if refreshRequired {
			var err error
			current, err = client.Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return err
			}
		}
		if current.GetUID() != observed.GetUID() ||
			current.GetGeneration() != observed.GetGeneration() {
			return biz.ErrResourceVersionConflict
		}

		setSpec(current)
		next, err := client.Update(ctx, current, metav1.UpdateOptions{})
		refreshRequired = err != nil
		if err != nil {
			return err
		}
		updated = next
		return nil
	})
	return updated, resourceError("replace", kind, name, err)
}

// deleteResource 保持初次读取的资源身份，并使用最新底层资源版本执行条件删除。
func deleteResource[T resourceObject[T]](
	ctx context.Context,
	client resourceClient[T],
	kind string,
	observed T,
) error {
	name := observed.GetName()
	current := observed.DeepCopy()
	refreshRequired := false
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if refreshRequired {
			var err error
			current, err = client.Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return err
			}
		}
		if current.GetUID() != observed.GetUID() ||
			current.GetGeneration() != observed.GetGeneration() {
			return biz.ErrResourceVersionConflict
		}

		uid := current.GetUID()
		resourceVersion := current.GetResourceVersion()
		err := client.Delete(ctx, name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID:             &uid,
				ResourceVersion: &resourceVersion,
			},
		})
		refreshRequired = err != nil
		return err
	})
	return resourceError("delete", kind, name, err)
}

// listByIDs 并发读取去重后的资源 ID。
// 不存在的资源不进入结果，其余错误会取消未完成的读取。
func listByIDs[T any](
	ctx context.Context,
	resourceIDs []string,
	get func(context.Context, string) (T, error),
) (map[string]T, error) {
	uniqueIDs := make([]string, 0, len(resourceIDs))
	seenIDs := make(map[string]bool, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		if seenIDs[resourceID] {
			continue
		}
		seenIDs[resourceID] = true
		uniqueIDs = append(uniqueIDs, resourceID)
	}

	resources := make([]T, len(uniqueIDs))
	found := make([]bool, len(uniqueIDs))
	group, lookupCtx := errgroup.WithContext(ctx)
	group.SetLimit(maxConcurrentResourceReads)
	for i, resourceID := range uniqueIDs {
		group.Go(func() error {
			resource, err := get(lookupCtx, resourceID)
			if errors.Is(err, biz.ErrResourceNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			resources[i] = resource
			found[i] = true
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	result := make(map[string]T, len(resources))
	for i, resource := range resources {
		if found[i] {
			result[uniqueIDs[i]] = resource
		}
	}
	return result, nil
}

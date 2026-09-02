package apiserver

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz/apperror"
	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// 单个管理请求最多占用八条并发读取，避免大 Route 耗尽 API Server 连接。
const maxConcurrentResourceReads = 8

// resourceObject 是支持元数据访问和强类型深拷贝的声明式资源。
type resourceObject[T any] interface {
	metav1.Object
	runtime.Object
	DeepCopy() T
}

// resourceClient 是声明式资源 Store 使用的 Kubernetes typed client 能力。
type resourceClient[Object, List any] interface {
	Create(ctx context.Context, object Object, options metav1.CreateOptions) (Object, error)
	Get(ctx context.Context, name string, options metav1.GetOptions) (Object, error)
	List(ctx context.Context, options metav1.ListOptions) (List, error)
	Update(ctx context.Context, object Object, options metav1.UpdateOptions) (Object, error)
	Delete(ctx context.Context, name string, options metav1.DeleteOptions) error
}

// resourceStore 实现声明式资源 Store 共享的 CRUD 行为。
type resourceStore[Item any, Object resourceObject[Object], List any, Spec any] struct {
	singularName string
	pluralName   string
	client       resourceClient[Object, List]
	unpackList   func(List) ([]Item, string)
	newObject    func(string, Spec) Object
	setSpec      func(Object, Spec)
}

// ListPage 分页返回声明式资源。
func (s *resourceStore[Item, Object, List, Spec]) ListPage(
	ctx context.Context,
	page pagination.Request,
) (pagination.Result[Item], error) {
	list, err := s.client.List(ctx, metav1.ListOptions{
		Limit:    page.Limit,
		Continue: page.Cursor,
	})
	if err != nil {
		return pagination.Result[Item]{}, listError(s.pluralName, err)
	}
	items, nextCursor := s.unpackList(list)
	return pagination.Result[Item]{Items: items, NextCursor: nextCursor}, nil
}

// Get 返回指定声明式资源。
func (s *resourceStore[Item, Object, List, Spec]) Get(
	ctx context.Context,
	resourceID string,
) (Object, error) {
	object, err := s.client.Get(ctx, resourceID, metav1.GetOptions{})
	return object, resourceError("get", s.singularName, resourceID, err)
}

// ListByIDs 返回当前存在的指定声明式资源。
func (s *resourceStore[Item, Object, List, Spec]) ListByIDs(
	ctx context.Context,
	resourceIDs []string,
) (map[string]Object, error) {
	return listByIDs(ctx, resourceIDs, s.Get)
}

// Create 创建声明式资源。
func (s *resourceStore[Item, Object, List, Spec]) Create(
	ctx context.Context,
	resourceID string,
	spec Spec,
) (Object, error) {
	created, err := s.client.Create(
		ctx,
		s.newObject(resourceID, spec),
		metav1.CreateOptions{},
	)
	return created, resourceError("create", s.singularName, resourceID, err)
}

// ReplaceSpec 完整替换声明式资源配置。
func (s *resourceStore[Item, Object, List, Spec]) ReplaceSpec(
	ctx context.Context,
	observed Object,
	spec Spec,
) (Object, error) {
	resourceID := observed.GetName()
	var updated Object
	err := s.retryResourceMutation(ctx, observed, func(current Object) error {
		s.setSpec(current, spec)
		next, err := s.client.Update(ctx, current, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		updated = next
		return nil
	})
	return updated, resourceError("replace", s.singularName, resourceID, err)
}

// Delete 删除声明式资源。
func (s *resourceStore[Item, Object, List, Spec]) Delete(
	ctx context.Context,
	observed Object,
) error {
	resourceID := observed.GetName()
	err := s.retryResourceMutation(ctx, observed, func(current Object) error {
		uid := current.GetUID()
		resourceVersion := current.GetResourceVersion()
		return s.client.Delete(ctx, resourceID, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID:             &uid,
				ResourceVersion: &resourceVersion,
			},
		})
	})
	return resourceError("delete", s.singularName, resourceID, err)
}

// retryResourceMutation 在资源身份和配置版本未变化时重试底层资源版本冲突。
func (s *resourceStore[Item, Object, List, Spec]) retryResourceMutation(
	ctx context.Context,
	observed Object,
	mutate func(Object) error,
) error {
	resourceID := observed.GetName()
	current := observed.DeepCopy()
	refreshRequired := false
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if refreshRequired {
			var err error
			current, err = s.client.Get(ctx, resourceID, metav1.GetOptions{})
			if err != nil {
				return err
			}
		}
		if current.GetUID() != observed.GetUID() ||
			current.GetGeneration() != observed.GetGeneration() {
			return apperror.ResourceVersionConflict()
		}

		err := mutate(current)
		refreshRequired = err != nil
		return err
	})
}

func newResource[T resourceObject[T]](resourceID string, kind resource.Kind, object T) T {
	object.SetName(resourceID)
	object.GetObjectKind().SetGroupVersionKind(
		resource.SchemeGroupVersion.WithKind(string(kind)),
	)
	return object
}

func listError(resourceName string, err error) error {
	if err == nil {
		return nil
	}
	if apierrors.IsBadRequest(err) || apierrors.IsResourceExpired(err) {
		return apperror.InvalidCursor(err)
	}
	return fmt.Errorf("list %s: %w", resourceName, translateResourceError(err))
}

func resourceError(operation, resourceName, resourceID string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(
		"%s %s %q: %w",
		operation,
		resourceName,
		resourceID,
		translateResourceError(err),
	)
}

func translateResourceError(err error) error {
	var domainErr error
	switch {
	case apierrors.IsNotFound(err):
		domainErr = apperror.ResourceNotFound()
	case apierrors.IsAlreadyExists(err):
		domainErr = apperror.ResourceAlreadyExists()
	case apierrors.IsConflict(err):
		domainErr = apperror.ResourceVersionConflict()
	case apierrors.IsInvalid(err):
		return apperror.InvalidResource(err)
	default:
		return err
	}
	return fmt.Errorf("%w: %w", domainErr, err)
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
			if errors.Is(err, apperror.ResourceNotFound()) {
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

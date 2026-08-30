package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
)

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

// updateResource 在资源身份和配置版本未变化时重试底层资源版本冲突。
func updateResource[T resourceObject[T]](
	ctx context.Context,
	client resourceClient[T],
	observed T,
	mutate func(T),
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

		mutate(current)
		next, err := client.Update(ctx, current, metav1.UpdateOptions{})
		refreshRequired = err != nil
		if err != nil {
			return err
		}
		updated = next
		return nil
	})
	return updated, err
}

// deleteResource 保持初次读取的资源身份，并使用最新底层资源版本执行条件删除。
func deleteResource[T resourceObject[T]](
	ctx context.Context,
	client resourceClient[T],
	observed T,
) error {
	name := observed.GetName()
	current := observed.DeepCopy()
	refreshRequired := false
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
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
}

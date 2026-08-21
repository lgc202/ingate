package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// CallerRepository 读写 Caller 声明式资源
type CallerRepository struct {
	client clientset.Interface
}

// NewCallerRepository 创建 Caller Repository
func NewCallerRepository(client clientset.Interface) *CallerRepository {
	return &CallerRepository{client: client}
}

// ListPage 分页查询 Caller 列表
func (r *CallerRepository) ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Caller], error) {
	callers, err := r.client.GatewayV1().Callers().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.Caller]{}, listError("callers", err)
	}
	return biz.PageResult[resource.Caller]{Items: callers.Items, NextCursor: callers.Continue}, nil
}

// Get 查询单个 Caller
func (r *CallerRepository) Get(ctx context.Context, name string) (*resource.Caller, error) {
	caller, err := r.client.GatewayV1().Callers().Get(ctx, name, metav1.GetOptions{})
	return caller, resourceError("get", "caller", name, err)
}

// Create 创建 Caller
func (r *CallerRepository) Create(ctx context.Context, name string, spec resource.CallerSpec) (*resource.Caller, error) {
	caller := &resource.Caller{
		TypeMeta:   metav1.TypeMeta{APIVersion: resource.SchemeGroupVersion.String(), Kind: string(resource.KindCaller)},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
	created, err := r.client.GatewayV1().Callers().Create(ctx, caller, metav1.CreateOptions{})
	return created, resourceError("create", "caller", name, err)
}

// Update 更新 Caller，并只重试其他写入导致的 ResourceVersion 冲突
func (r *CallerRepository) Update(
	ctx context.Context,
	name string,
	generation int64,
	spec resource.CallerSpec,
) (*resource.Caller, error) {
	var updated *resource.Caller
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().Callers().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		updated, err = r.client.GatewayV1().Callers().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return updated, resourceError("update", "caller", name, err)
}

// Delete 删除 Caller
func (r *CallerRepository) Delete(ctx context.Context, name string, generation int64) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().Callers().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		resourceVersion := current.ResourceVersion
		return r.client.GatewayV1().Callers().Delete(ctx, name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{ResourceVersion: &resourceVersion},
		})
	})
	return resourceError("delete", "caller", name, err)
}

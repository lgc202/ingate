package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// UpstreamRepository 读写 Upstream 声明式资源
type UpstreamRepository struct {
	client clientset.Interface
}

// NewUpstreamRepository 创建 Upstream Repository
func NewUpstreamRepository(client clientset.Interface) *UpstreamRepository {
	return &UpstreamRepository{client: client}
}

// ListPage 分页查询 Upstream 列表
func (r *UpstreamRepository) ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Upstream], error) {
	upstreams, err := r.client.GatewayV1().Upstreams().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.Upstream]{}, listError("upstreams", err)
	}
	return biz.PageResult[resource.Upstream]{Items: upstreams.Items, NextCursor: upstreams.Continue}, nil
}

// Get 查询单个 Upstream
func (r *UpstreamRepository) Get(ctx context.Context, name string) (*resource.Upstream, error) {
	upstream, err := r.client.GatewayV1().Upstreams().Get(ctx, name, metav1.GetOptions{})
	return upstream, resourceError("get", "upstream", name, err)
}

// Create 创建 Upstream
func (r *UpstreamRepository) Create(ctx context.Context, name string, spec resource.UpstreamSpec) (*resource.Upstream, error) {
	upstream := &resource.Upstream{
		TypeMeta:   metav1.TypeMeta{APIVersion: resource.SchemeGroupVersion.String(), Kind: string(resource.KindUpstream)},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
	created, err := r.client.GatewayV1().Upstreams().Create(ctx, upstream, metav1.CreateOptions{})
	return created, resourceError("create", "upstream", name, err)
}

// Update 更新 Upstream，并只重试 Controller 写 status 导致的 ResourceVersion 冲突
func (r *UpstreamRepository) Update(
	ctx context.Context,
	name string,
	generation int64,
	spec resource.UpstreamSpec,
) (*resource.Upstream, error) {
	var updated *resource.Upstream
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().Upstreams().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		updated, err = r.client.GatewayV1().Upstreams().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return updated, resourceError("update", "upstream", name, err)
}

// Delete 删除 Upstream
func (r *UpstreamRepository) Delete(ctx context.Context, name string, generation int64) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().Upstreams().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		resourceVersion := current.ResourceVersion
		return r.client.GatewayV1().Upstreams().Delete(ctx, name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{ResourceVersion: &resourceVersion},
		})
	})
	return resourceError("delete", "upstream", name, err)
}

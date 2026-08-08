package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// UpstreamRepository 读写 Upstream 声明式资源
type UpstreamRepository struct {
	client clientset.Interface
}

// NewUpstream 创建 Upstream Repository
func NewUpstream(client clientset.Interface) *UpstreamRepository {
	return &UpstreamRepository{client: client}
}

// List 查询 Upstream 列表
func (r *UpstreamRepository) List(ctx context.Context) ([]resource.Upstream, error) {
	upstreams, err := r.client.GatewayV1().Upstreams().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, resourceError("list", "upstreams", "", err)
	}
	return upstreams.Items, nil
}

// Get 查询单个 Upstream
func (r *UpstreamRepository) Get(ctx context.Context, name string) (*resource.Upstream, error) {
	upstream, err := r.client.GatewayV1().Upstreams().Get(ctx, name, metav1.GetOptions{})
	return upstream, resourceError("get", "upstream", name, err)
}

// Create 创建 Upstream
func (r *UpstreamRepository) Create(ctx context.Context, id string, spec resource.UpstreamSpec) error {
	upstream := &resource.Upstream{
		TypeMeta:   metav1.TypeMeta{APIVersion: resource.SchemeGroupVersion.String(), Kind: string(resource.KindUpstream)},
		ObjectMeta: metav1.ObjectMeta{Name: id},
		Spec:       spec,
	}
	_, err := r.client.GatewayV1().Upstreams().Create(ctx, upstream, metav1.CreateOptions{})
	return resourceError("create", "upstream", id, err)
}

// Update 更新 Upstream，并只重试 Controller 写 status 导致的 ResourceVersion 冲突
func (r *UpstreamRepository) Update(ctx context.Context, id string, generation int64, spec resource.UpstreamSpec) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().Upstreams().Get(ctx, id, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		_, err = r.client.GatewayV1().Upstreams().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return resourceError("update", "upstream", id, err)
}

// Delete 删除 Upstream
func (r *UpstreamRepository) Delete(ctx context.Context, name string) error {
	err := r.client.GatewayV1().Upstreams().Delete(ctx, name, metav1.DeleteOptions{})
	return resourceError("delete", "upstream", name, err)
}

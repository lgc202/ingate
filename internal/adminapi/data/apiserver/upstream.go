package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
func (r *UpstreamRepository) List(ctx context.Context) (*resource.UpstreamList, error) {
	return r.client.GatewayV1().Upstreams().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 Upstream
func (r *UpstreamRepository) Get(ctx context.Context, name string) (*resource.Upstream, error) {
	return r.client.GatewayV1().Upstreams().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 Upstream
func (r *UpstreamRepository) Create(ctx context.Context, upstream *resource.Upstream) (*resource.Upstream, error) {
	return r.client.GatewayV1().Upstreams().Create(ctx, upstream, metav1.CreateOptions{})
}

// Update 更新 Upstream
func (r *UpstreamRepository) Update(ctx context.Context, upstream *resource.Upstream) (*resource.Upstream, error) {
	return r.client.GatewayV1().Upstreams().Update(ctx, upstream, metav1.UpdateOptions{})
}

// Delete 删除 Upstream
func (r *UpstreamRepository) Delete(ctx context.Context, name string) error {
	return r.client.GatewayV1().Upstreams().Delete(ctx, name, metav1.DeleteOptions{})
}

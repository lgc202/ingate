package upstream

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 读取 Upstream 资源
type Store struct {
	client clientset.Interface
}

// New 创建 Upstream store
func New(client clientset.Interface) *Store {
	return &Store{client: client}
}

// List 查询 Upstream 列表
func (s *Store) List(ctx context.Context) (*resource.UpstreamList, error) {
	return s.client.GatewayV1().Upstreams().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 Upstream
func (s *Store) Get(ctx context.Context, name string) (*resource.Upstream, error) {
	return s.client.GatewayV1().Upstreams().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 Upstream
func (s *Store) Create(ctx context.Context, upstream *resource.Upstream) (*resource.Upstream, error) {
	return s.client.GatewayV1().Upstreams().Create(ctx, upstream, metav1.CreateOptions{})
}

// Update 更新 Upstream
func (s *Store) Update(ctx context.Context, upstream *resource.Upstream) (*resource.Upstream, error) {
	return s.client.GatewayV1().Upstreams().Update(ctx, upstream, metav1.UpdateOptions{})
}

// Delete 删除 Upstream
func (s *Store) Delete(ctx context.Context, name string) error {
	return s.client.GatewayV1().Upstreams().Delete(ctx, name, metav1.DeleteOptions{})
}

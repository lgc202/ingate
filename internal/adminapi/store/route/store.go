package route

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 读取 Route 资源
type Store struct {
	client clientset.Interface
}

// New 创建 Route store
func New(client clientset.Interface) *Store {
	return &Store{client: client}
}

// List 查询 Route 列表
func (s *Store) List(ctx context.Context) (*resource.RouteList, error) {
	return s.client.GatewayV1().Routes().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 Route
func (s *Store) Get(ctx context.Context, name string) (*resource.Route, error) {
	return s.client.GatewayV1().Routes().Get(ctx, name, metav1.GetOptions{})
}

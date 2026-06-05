package runtimegroup

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 读取 RuntimeGroup 资源
type Store struct {
	client clientset.Interface
}

// New 创建 RuntimeGroup store
func New(client clientset.Interface) *Store {
	return &Store{client: client}
}

// List 查询 RuntimeGroup 列表
func (s *Store) List(ctx context.Context) (*resource.RuntimeGroupList, error) {
	return s.client.GatewayV1().RuntimeGroups().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 RuntimeGroup
func (s *Store) Get(ctx context.Context, name string) (*resource.RuntimeGroup, error) {
	return s.client.GatewayV1().RuntimeGroups().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 RuntimeGroup
func (s *Store) Create(ctx context.Context, runtimeGroup *resource.RuntimeGroup) (*resource.RuntimeGroup, error) {
	return s.client.GatewayV1().RuntimeGroups().Create(ctx, runtimeGroup, metav1.CreateOptions{})
}

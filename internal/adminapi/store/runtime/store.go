package runtime

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 读取 RuntimeSnapshot 资源
type Store struct {
	client clientset.Interface
}

// New 创建 RuntimeSnapshot store
func New(client clientset.Interface) *Store {
	return &Store{client: client}
}

// List 查询 RuntimeSnapshot 列表
func (s *Store) List(ctx context.Context) (*resource.RuntimeSnapshotList, error) {
	return s.client.GatewayV1().RuntimeSnapshots().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 RuntimeSnapshot
func (s *Store) Get(ctx context.Context, name string) (*resource.RuntimeSnapshot, error) {
	return s.client.GatewayV1().RuntimeSnapshots().Get(ctx, name, metav1.GetOptions{})
}

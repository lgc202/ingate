package redisstore

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 读写 RedisStore 资源
type Store struct {
	client clientset.Interface
}

// New 创建 RedisStore store
func New(client clientset.Interface) *Store {
	return &Store{client: client}
}

// List 查询 RedisStore 列表
func (s *Store) List(ctx context.Context) (*resource.RedisStoreList, error) {
	return s.client.GatewayV1().RedisStores().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 RedisStore
func (s *Store) Get(ctx context.Context, name string) (*resource.RedisStore, error) {
	return s.client.GatewayV1().RedisStores().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 RedisStore
func (s *Store) Create(ctx context.Context, store *resource.RedisStore) (*resource.RedisStore, error) {
	return s.client.GatewayV1().RedisStores().Create(ctx, store, metav1.CreateOptions{})
}

// Update 更新 RedisStore
func (s *Store) Update(ctx context.Context, store *resource.RedisStore) (*resource.RedisStore, error) {
	return s.client.GatewayV1().RedisStores().Update(ctx, store, metav1.UpdateOptions{})
}

// Delete 删除 RedisStore
func (s *Store) Delete(ctx context.Context, name string) error {
	return s.client.GatewayV1().RedisStores().Delete(ctx, name, metav1.DeleteOptions{})
}

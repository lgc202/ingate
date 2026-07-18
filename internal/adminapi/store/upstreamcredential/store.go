// Package upstreamcredential 封装 UpstreamCredential 声明式资源读写
package upstreamcredential

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 读写 UpstreamCredential 资源
type Store struct {
	client clientset.Interface
}

// New 创建 UpstreamCredential store
func New(client clientset.Interface) *Store {
	return &Store{client: client}
}

// List 查询 UpstreamCredential 列表
func (s *Store) List(ctx context.Context) (*resource.UpstreamCredentialList, error) {
	return s.client.GatewayV1().UpstreamCredentials().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 UpstreamCredential
func (s *Store) Get(ctx context.Context, name string) (*resource.UpstreamCredential, error) {
	return s.client.GatewayV1().UpstreamCredentials().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 UpstreamCredential
func (s *Store) Create(ctx context.Context, credential *resource.UpstreamCredential) (*resource.UpstreamCredential, error) {
	return s.client.GatewayV1().UpstreamCredentials().Create(ctx, credential, metav1.CreateOptions{})
}

// Update 更新 UpstreamCredential
func (s *Store) Update(ctx context.Context, credential *resource.UpstreamCredential) (*resource.UpstreamCredential, error) {
	return s.client.GatewayV1().UpstreamCredentials().Update(ctx, credential, metav1.UpdateOptions{})
}

// Delete 删除 UpstreamCredential
func (s *Store) Delete(ctx context.Context, name string) error {
	return s.client.GatewayV1().UpstreamCredentials().Delete(ctx, name, metav1.DeleteOptions{})
}

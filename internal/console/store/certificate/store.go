// Package certificate 封装 Certificate 声明式资源读写
package certificate

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 读写 Certificate 资源
type Store struct {
	client clientset.Interface
}

// New 创建 Certificate store
func New(client clientset.Interface) *Store {
	return &Store{client: client}
}

// List 查询 Certificate 列表
func (s *Store) List(ctx context.Context) (*resource.CertificateList, error) {
	return s.client.GatewayV1().Certificates().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 Certificate
func (s *Store) Get(ctx context.Context, name string) (*resource.Certificate, error) {
	return s.client.GatewayV1().Certificates().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 Certificate
func (s *Store) Create(ctx context.Context, certificate *resource.Certificate) (*resource.Certificate, error) {
	return s.client.GatewayV1().Certificates().Create(ctx, certificate, metav1.CreateOptions{})
}

// Update 更新 Certificate
func (s *Store) Update(ctx context.Context, certificate *resource.Certificate) (*resource.Certificate, error) {
	return s.client.GatewayV1().Certificates().Update(ctx, certificate, metav1.UpdateOptions{})
}

// Delete 删除 Certificate
func (s *Store) Delete(ctx context.Context, name string) error {
	return s.client.GatewayV1().Certificates().Delete(ctx, name, metav1.DeleteOptions{})
}

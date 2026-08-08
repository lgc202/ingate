// Package apiserver 通过 Ingate API Server 读写声明式资源
package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// CertificateRepository 读写 Certificate 声明式资源
type CertificateRepository struct {
	client clientset.Interface
}

// NewCertificate 创建 Certificate Repository
func NewCertificate(client clientset.Interface) *CertificateRepository {
	return &CertificateRepository{client: client}
}

// List 查询 Certificate 列表
func (r *CertificateRepository) List(ctx context.Context) (*resource.CertificateList, error) {
	return r.client.GatewayV1().Certificates().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 Certificate
func (r *CertificateRepository) Get(ctx context.Context, name string) (*resource.Certificate, error) {
	return r.client.GatewayV1().Certificates().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 Certificate
func (r *CertificateRepository) Create(ctx context.Context, certificate *resource.Certificate) (*resource.Certificate, error) {
	return r.client.GatewayV1().Certificates().Create(ctx, certificate, metav1.CreateOptions{})
}

// Update 更新 Certificate
func (r *CertificateRepository) Update(ctx context.Context, certificate *resource.Certificate) (*resource.Certificate, error) {
	return r.client.GatewayV1().Certificates().Update(ctx, certificate, metav1.UpdateOptions{})
}

// Delete 删除 Certificate
func (r *CertificateRepository) Delete(ctx context.Context, name string) error {
	return r.client.GatewayV1().Certificates().Delete(ctx, name, metav1.DeleteOptions{})
}

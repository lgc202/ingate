// Package apiserver 通过 Ingate API Server 读写声明式资源
package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
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
func (r *CertificateRepository) List(ctx context.Context) ([]resource.Certificate, error) {
	certificates, err := r.client.GatewayV1().Certificates().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, resourceError("list", "certificates", "", err)
	}
	return certificates.Items, nil
}

// Get 查询单个 Certificate
func (r *CertificateRepository) Get(ctx context.Context, name string) (*resource.Certificate, error) {
	certificate, err := r.client.GatewayV1().Certificates().Get(ctx, name, metav1.GetOptions{})
	return certificate, resourceError("get", "certificate", name, err)
}

// Create 创建 Certificate
func (r *CertificateRepository) Create(ctx context.Context, id string, spec resource.CertificateSpec) error {
	certificate := &resource.Certificate{
		TypeMeta:   metav1.TypeMeta{APIVersion: resource.SchemeGroupVersion.String(), Kind: string(resource.KindCertificate)},
		ObjectMeta: metav1.ObjectMeta{Name: id},
		Spec:       spec,
	}
	_, err := r.client.GatewayV1().Certificates().Create(ctx, certificate, metav1.CreateOptions{})
	return resourceError("create", "certificate", id, err)
}

// Update 更新 Certificate，并只重试 Controller 写 status 导致的 ResourceVersion 冲突
func (r *CertificateRepository) Update(ctx context.Context, id string, generation int64, spec resource.CertificateSpec) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().Certificates().Get(ctx, id, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		_, err = r.client.GatewayV1().Certificates().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return resourceError("update", "certificate", id, err)
}

// Delete 删除 Certificate
func (r *CertificateRepository) Delete(ctx context.Context, name string) error {
	err := r.client.GatewayV1().Certificates().Delete(ctx, name, metav1.DeleteOptions{})
	return resourceError("delete", "certificate", name, err)
}

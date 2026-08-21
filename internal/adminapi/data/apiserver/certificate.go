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

// NewCertificateRepository 创建 Certificate Repository
func NewCertificateRepository(client clientset.Interface) *CertificateRepository {
	return &CertificateRepository{client: client}
}

// ListPage 分页查询 Certificate 列表
func (r *CertificateRepository) ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Certificate], error) {
	certificates, err := r.client.GatewayV1().Certificates().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.Certificate]{}, listError("certificates", err)
	}
	return biz.PageResult[resource.Certificate]{Items: certificates.Items, NextCursor: certificates.Continue}, nil
}

// Get 查询单个 Certificate
func (r *CertificateRepository) Get(ctx context.Context, name string) (*resource.Certificate, error) {
	certificate, err := r.client.GatewayV1().Certificates().Get(ctx, name, metav1.GetOptions{})
	return certificate, resourceError("get", "certificate", name, err)
}

// Create 创建 Certificate
func (r *CertificateRepository) Create(ctx context.Context, name string, spec resource.CertificateSpec) (*resource.Certificate, error) {
	certificate := &resource.Certificate{
		TypeMeta:   metav1.TypeMeta{APIVersion: resource.SchemeGroupVersion.String(), Kind: string(resource.KindCertificate)},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
	created, err := r.client.GatewayV1().Certificates().Create(ctx, certificate, metav1.CreateOptions{})
	return created, resourceError("create", "certificate", name, err)
}

// Update 更新 Certificate，并只重试 Controller 写 status 导致的 ResourceVersion 冲突
func (r *CertificateRepository) Update(
	ctx context.Context,
	name string,
	generation int64,
	spec resource.CertificateSpec,
) (*resource.Certificate, error) {
	var updated *resource.Certificate
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().Certificates().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		updated, err = r.client.GatewayV1().Certificates().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return updated, resourceError("update", "certificate", name, err)
}

// Delete 删除 Certificate
func (r *CertificateRepository) Delete(ctx context.Context, name string, generation int64) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().Certificates().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		resourceVersion := current.ResourceVersion
		return r.client.GatewayV1().Certificates().Delete(ctx, name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{ResourceVersion: &resourceVersion},
		})
	})
	return resourceError("delete", "certificate", name, err)
}

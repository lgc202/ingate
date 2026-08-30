package apiserver

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// CertificateStore 读写 Certificate 声明式资源。
type CertificateStore struct {
	client clientset.Interface
}

// NewCertificateStore 创建 Certificate Store。
func NewCertificateStore(client clientset.Interface) *CertificateStore {
	return &CertificateStore{client: client}
}

// ListPage 分页返回 Certificate。
func (s *CertificateStore) ListPage(
	ctx context.Context,
	page biz.PageRequest,
) (biz.PageResult[resource.Certificate], error) {
	certificates, err := s.client.GatewayV1().Certificates().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.Certificate]{}, listError("certificates", err)
	}
	return biz.PageResult[resource.Certificate]{
		Items:      certificates.Items,
		NextCursor: certificates.Continue,
	}, nil
}

// Get 返回指定 Certificate。
func (s *CertificateStore) Get(ctx context.Context, certificateID string) (*resource.Certificate, error) {
	certificate, err := s.client.GatewayV1().Certificates().Get(ctx, certificateID, metav1.GetOptions{})
	return certificate, resourceError("get", "certificate", certificateID, err)
}

// Create 创建 Certificate。
func (s *CertificateStore) Create(
	ctx context.Context,
	certificateID string,
	spec resource.CertificateSpec,
) (*resource.Certificate, error) {
	certificate := &resource.Certificate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindCertificate),
		},
		ObjectMeta: metav1.ObjectMeta{Name: certificateID},
		Spec:       spec,
	}
	created, err := s.client.GatewayV1().Certificates().Create(ctx, certificate, metav1.CreateOptions{})
	return created, resourceError("create", "certificate", certificateID, err)
}

// ReplaceSpec 完整替换 Certificate 配置。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *CertificateStore) ReplaceSpec(
	ctx context.Context,
	observed *resource.Certificate,
	spec resource.CertificateSpec,
) (*resource.Certificate, error) {
	certificateID := observed.Name
	updated, err := updateResource(
		ctx,
		s.client.GatewayV1().Certificates(),
		observed,
		func(certificate *resource.Certificate) { certificate.Spec = spec },
	)
	if apierrors.IsConflict(err) {
		return nil, fmt.Errorf("replace certificate %q after conflict retries: %w", certificateID, err)
	}
	return updated, resourceError("replace", "certificate", certificateID, err)
}

// Delete 删除 Certificate。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *CertificateStore) Delete(
	ctx context.Context,
	observed *resource.Certificate,
) error {
	certificateID := observed.Name
	err := deleteResource(ctx, s.client.GatewayV1().Certificates(), observed)
	if apierrors.IsConflict(err) {
		return fmt.Errorf("delete certificate %q after conflict retries: %w", certificateID, err)
	}
	return resourceError("delete", "certificate", certificateID, err)
}

package certificate

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	certificatestore "github.com/lgc202/ingate/internal/adminapi/store/certificate"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Service 承载 Certificate 管理用例
type Service struct {
	store    *certificatestore.Store
	gateways *gatewaystore.Store
}

// New 创建 Certificate service
func New(store *certificatestore.Store, gateways *gatewaystore.Store) *Service {
	return &Service{store: store, gateways: gateways}
}

// List 查询 Certificate 列表
func (s *Service) List(ctx context.Context) ([]resource.Certificate, error) {
	certificates, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	return certificates.Items, nil
}

// Get 查询单个 Certificate
func (s *Service) Get(ctx context.Context, certificateID string) (*resource.Certificate, error) {
	certificate, err := s.store.Get(ctx, certificateID)
	if err != nil {
		return nil, err
	}
	return certificate, nil
}

// Create 创建 Certificate
func (s *Service) Create(ctx context.Context, params CreateParams) (string, error) {
	if err := s.validateNameUnique(ctx, params.Name, ""); err != nil {
		return "", err
	}
	certificate := certificateResource(uuid.NewString(), "", params.CertificateParams)
	created, err := s.store.Create(ctx, certificate)
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

// Update 更新 Certificate
func (s *Service) Update(ctx context.Context, certificateID string, params UpdateParams) error {
	current, err := s.store.Get(ctx, certificateID)
	if err != nil {
		return err
	}
	if params.Version == "" {
		return xerrors.NewUserError("证书版本不能为空")
	}
	if params.Version != current.ResourceVersion {
		return xerrors.NewUserError(fmt.Sprintf("%s %q 已被更新，请刷新后重试", resource.ResourceCertificates, certificateID))
	}
	if err := s.validateNameUnique(ctx, params.Name, certificateID); err != nil {
		return err
	}

	next := current.DeepCopy()
	next.Spec.DisplayName = params.Name
	next.Spec.Description = params.Description
	if params.CertificatePEM != "" {
		next.Spec.CertificatePEM = params.CertificatePEM
		next.Spec.PrivateKeyPEM = params.PrivateKeyPEM
	}
	_, err = s.store.Update(ctx, next)
	return err
}

// Delete 删除 Certificate，仍被 Gateway 引用时拒绝删除
func (s *Service) Delete(ctx context.Context, certificateID string) error {
	gateways, err := s.gateways.List(ctx)
	if err != nil {
		return err
	}
	for _, gateway := range gateways.Items {
		for _, listener := range gateway.Spec.Listeners {
			if listener.CertificateRef == certificateID {
				return xerrors.NewUserError(fmt.Sprintf("证书仍被网关 %q 引用", gateway.Spec.DisplayName))
			}
		}
	}
	return s.store.Delete(ctx, certificateID)
}

func (s *Service) validateNameUnique(ctx context.Context, name, excludeID string) error {
	certificates, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	for _, certificate := range certificates.Items {
		if certificate.Name == excludeID {
			continue
		}
		if certificate.Spec.DisplayName == name {
			return xerrors.NewUserError(fmt.Sprintf("证书名称 %q 已存在", name))
		}
	}
	return nil
}

func certificateResource(id, version string, params CertificateParams) *resource.Certificate {
	return &resource.Certificate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindCertificate),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            id,
			ResourceVersion: version,
		},
		Spec: resource.CertificateSpec{
			DisplayName:    params.Name,
			Description:    params.Description,
			CertificatePEM: params.CertificatePEM,
			PrivateKeyPEM:  params.PrivateKeyPEM,
		},
	}
}

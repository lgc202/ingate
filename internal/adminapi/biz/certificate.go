package biz

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// CertificateRepository 定义 Certificate 用例需要的持久化能力
type CertificateRepository interface {
	List(context.Context) ([]resource.Certificate, error)
	Get(context.Context, string) (*resource.Certificate, error)
	Create(context.Context, string, resource.CertificateSpec) error
	Update(context.Context, string, int64, resource.CertificateSpec) error
	Delete(context.Context, string) error
}

// CertificateUsecase 承载 Certificate 管理用例
type CertificateUsecase struct {
	repository CertificateRepository
	gateways   GatewayRepository
}

// NewCertificateUsecase 创建证书管理用例
func NewCertificateUsecase(repository CertificateRepository, gateways GatewayRepository) *CertificateUsecase {
	return &CertificateUsecase{repository: repository, gateways: gateways}
}

// List 查询 Certificate 列表
func (s *CertificateUsecase) List(ctx context.Context) ([]resource.Certificate, error) {
	certificates, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	return certificates, nil
}

// Get 查询单个 Certificate
func (s *CertificateUsecase) Get(ctx context.Context, certificateID string) (*resource.Certificate, error) {
	certificate, err := s.repository.Get(ctx, certificateID)
	if err != nil {
		return nil, err
	}
	return certificate, nil
}

// Create 创建 Certificate
func (s *CertificateUsecase) Create(ctx context.Context, spec resource.CertificateSpec) (string, error) {
	if err := s.validateNameUnique(ctx, spec.DisplayName, ""); err != nil {
		return "", err
	}
	id := uuid.NewString()
	if err := s.repository.Create(ctx, id, spec); err != nil {
		return "", err
	}
	return id, nil
}

// Update 更新 Certificate
func (s *CertificateUsecase) Update(ctx context.Context, certificateID, version string, spec resource.CertificateSpec) error {
	current, err := s.repository.Get(ctx, certificateID)
	if err != nil {
		return err
	}
	if version != strconv.FormatInt(current.Generation, 10) {
		return NewUserError(fmt.Sprintf("证书 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
	}
	if err := s.validateNameUnique(ctx, spec.DisplayName, certificateID); err != nil {
		return err
	}
	if spec.CertificatePEM == "" {
		spec.CertificatePEM = current.Spec.CertificatePEM
		spec.PrivateKeyPEM = current.Spec.PrivateKeyPEM
	}
	if err := s.repository.Update(ctx, certificateID, current.Generation, spec); err != nil {
		if errors.Is(err, ErrResourceVersionConflict) {
			return NewUserError(fmt.Sprintf("证书 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// Delete 删除 Certificate，仍被 Gateway 引用时拒绝删除
func (s *CertificateUsecase) Delete(ctx context.Context, certificateID string) error {
	gateways, err := s.gateways.List(ctx)
	if err != nil {
		return err
	}
	for _, gateway := range gateways {
		for _, listener := range gateway.Spec.Listeners {
			if listener.CertificateRef == certificateID {
				return NewUserError(fmt.Sprintf("证书仍被网关 %q 引用", gateway.Spec.DisplayName))
			}
		}
	}
	return s.repository.Delete(ctx, certificateID)
}

func (s *CertificateUsecase) validateNameUnique(ctx context.Context, name, excludeID string) error {
	certificates, err := s.repository.List(ctx)
	if err != nil {
		return err
	}
	for _, certificate := range certificates {
		if certificate.Name == excludeID {
			continue
		}
		if certificate.Spec.DisplayName == name {
			return NewUserError(fmt.Sprintf("证书名称 %q 已存在", name))
		}
	}
	return nil
}

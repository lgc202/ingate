// Package certificate 处理 Certificate 的管理规则和资源协作
package certificate

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Repository 定义 Certificate 管理需要的持久化能力
type Repository interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Certificate], error)
	Get(ctx context.Context, certificateID string) (*resource.Certificate, error)
	Create(ctx context.Context, certificateID string, spec resource.CertificateSpec) (*resource.Certificate, error)
	Update(ctx context.Context, certificateID string, generation int64, spec resource.CertificateSpec) (*resource.Certificate, error)
	Delete(ctx context.Context, certificateID string, generation int64) error
}

// GatewayRepository 定义删除 Certificate 时需要的 Gateway 查询能力
type GatewayRepository interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Gateway], error)
}

// Service 协调 Certificate 的校验、引用约束和持久化
type Service struct {
	repository Repository
	gateways   GatewayRepository
}

// NewService 创建 Certificate 业务服务
func NewService(repository Repository, gateways GatewayRepository) *Service {
	return &Service{repository: repository, gateways: gateways}
}

// List 查询 Certificate 列表
func (s *Service) List(ctx context.Context, page biz.PageRequest, filter biz.ResourceFilter) (biz.PageResult[resource.Certificate], error) {
	return biz.FilterPage(ctx, page, s.repository.ListPage, func(certificate resource.Certificate) bool {
		status := biz.ResourceStatusFromConditions(certificate.Generation, certificate.Status.Conditions)
		return filter.Match(certificate.Spec.DisplayName, true, status)
	})
}

// Get 查询单个 Certificate
func (s *Service) Get(ctx context.Context, certificateID string) (*resource.Certificate, error) {
	return s.repository.Get(ctx, certificateID)
}

// Create 创建 Certificate
func (s *Service) Create(ctx context.Context, spec resource.CertificateSpec) (*resource.Certificate, error) {
	if err := s.ensureDisplayNameAvailable(ctx, "", spec.DisplayName); err != nil {
		return nil, err
	}
	return s.repository.Create(ctx, uuid.NewString(), spec)
}

// Update 使用配置版本乐观更新 Certificate
func (s *Service) Update(
	ctx context.Context,
	certificateID string,
	version int64,
	spec resource.CertificateSpec,
) (*resource.Certificate, error) {
	current, err := s.repository.Get(ctx, certificateID)
	if err != nil {
		return nil, err
	}
	if version != current.Generation {
		return nil, certificateVersionConflict(current)
	}
	if spec.DisplayName != current.Spec.DisplayName {
		if err := s.ensureDisplayNameAvailable(ctx, certificateID, spec.DisplayName); err != nil {
			return nil, err
		}
	}
	if spec.CertificatePEM == "" {
		spec.CertificatePEM = current.Spec.CertificatePEM
		spec.PrivateKeyPEM = current.Spec.PrivateKeyPEM
	}

	updated, err := s.repository.Update(ctx, certificateID, current.Generation, spec)
	if err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return nil, certificateVersionConflict(current)
		}
		return nil, err
	}
	return updated, nil
}

// Delete 删除 Certificate，仍被 Gateway 引用时拒绝删除
func (s *Service) Delete(ctx context.Context, certificateID string, version int64) error {
	current, err := s.repository.Get(ctx, certificateID)
	if err != nil {
		return err
	}
	if version != current.Generation {
		return certificateVersionConflict(current)
	}
	if err := s.ensureNotReferenced(ctx, current); err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, certificateID, current.Generation); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return certificateVersionConflict(current)
		}
		return err
	}
	return nil
}

func (s *Service) ensureDisplayNameAvailable(ctx context.Context, certificateID, displayName string) error {
	return biz.VisitPages(ctx, s.repository.ListPage, func(certificate resource.Certificate) (bool, error) {
		if certificate.Name != certificateID && certificate.Spec.DisplayName == displayName {
			return true, biz.NewRuleViolation(fmt.Sprintf("证书名称 %q 已存在", displayName))
		}
		return false, nil
	})
}

func certificateVersionConflict(certificate *resource.Certificate) error {
	return biz.NewVersionConflict(
		certificate.Name,
		fmt.Sprintf("证书 %q 已被其他用户修改，请刷新后重试", certificate.Spec.DisplayName),
	)
}

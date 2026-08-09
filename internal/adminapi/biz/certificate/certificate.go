// Package certificate 实现 Certificate 管理用例
package certificate

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Repository 定义 Certificate 用例需要的持久化能力
type Repository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.Certificate], error)
	Get(context.Context, string) (*resource.Certificate, error)
	Create(context.Context, string, resource.CertificateSpec) error
	Update(context.Context, string, int64, resource.CertificateSpec) error
	Delete(context.Context, string) error
}

// GatewayRepository 定义删除 Certificate 时需要的 Gateway 查询能力
type GatewayRepository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.Gateway], error)
}

// Usecase 承载 Certificate 管理用例
type Usecase struct {
	repository Repository
	gateways   GatewayRepository
}

// NewUsecase 创建证书管理用例
func NewUsecase(repository Repository, gateways GatewayRepository) *Usecase {
	return &Usecase{repository: repository, gateways: gateways}
}

// List 查询 Certificate 列表
func (u *Usecase) List(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Certificate], error) {
	return u.repository.ListPage(ctx, page)
}

// Get 查询单个 Certificate
func (u *Usecase) Get(ctx context.Context, certificateID string) (*resource.Certificate, error) {
	certificate, err := u.repository.Get(ctx, certificateID)
	if err != nil {
		return nil, err
	}
	return certificate, nil
}

// Create 创建 Certificate
func (u *Usecase) Create(ctx context.Context, spec resource.CertificateSpec) (string, error) {
	id := uuid.NewString()
	if err := u.repository.Create(ctx, id, spec); err != nil {
		return "", biz.DisplayNameConflict(err, "证书", spec.DisplayName)
	}
	return id, nil
}

// Update 更新 Certificate
func (u *Usecase) Update(ctx context.Context, certificateID, version string, spec resource.CertificateSpec) error {
	current, err := u.repository.Get(ctx, certificateID)
	if err != nil {
		return err
	}
	if version != strconv.FormatInt(current.Generation, 10) {
		return biz.NewUserError(fmt.Sprintf("证书 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
	}
	if spec.CertificatePEM == "" {
		spec.CertificatePEM = current.Spec.CertificatePEM
		spec.PrivateKeyPEM = current.Spec.PrivateKeyPEM
	}
	if err := u.repository.Update(ctx, certificateID, current.Generation, spec); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return biz.NewUserError(fmt.Sprintf("证书 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return biz.DisplayNameConflict(err, "证书", spec.DisplayName)
	}
	return nil
}

// Delete 删除 Certificate，仍被 Gateway 引用时拒绝删除
func (u *Usecase) Delete(ctx context.Context, certificateID string) error {
	if err := biz.VisitPages(ctx, u.gateways.ListPage, func(gateway resource.Gateway) (bool, error) {
		for _, listener := range gateway.Spec.Listeners {
			if listener.CertificateRef == certificateID {
				return true, biz.NewUserError(fmt.Sprintf("证书仍被网关 %q 引用", gateway.Spec.DisplayName))
			}
		}
		return false, nil
	}); err != nil {
		return err
	}
	return u.repository.Delete(ctx, certificateID)
}

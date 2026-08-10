// Package gateway 实现 Gateway 管理用例
package gateway

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Repository 定义 Gateway 用例需要的持久化能力
type Repository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.Gateway], error)
	Get(context.Context, string) (*resource.Gateway, error)
	Create(context.Context, string, resource.GatewaySpec) (*resource.Gateway, error)
	Update(context.Context, string, int64, resource.GatewaySpec) (*resource.Gateway, error)
	Delete(context.Context, string, int64) error
}

// RouteRepository 定义删除 Gateway 时需要的 Route 查询能力
type RouteRepository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.Route], error)
}

// CertificateRepository 定义 Gateway 校验证书引用时需要的查询能力
type CertificateRepository interface {
	Get(context.Context, string) (*resource.Certificate, error)
}

// Usecase 承载 Gateway 管理用例
type Usecase struct {
	repository   Repository
	routes       RouteRepository
	certificates CertificateRepository
	policyUsage  *biz.PolicyUsageFinder
}

// NewUsecase 创建网关管理用例
func NewUsecase(
	repository Repository,
	routes RouteRepository,
	certificates CertificateRepository,
	policyUsage *biz.PolicyUsageFinder,
) *Usecase {
	return &Usecase{
		repository:   repository,
		routes:       routes,
		certificates: certificates,
		policyUsage:  policyUsage,
	}
}

// List 查询 Gateway 列表
func (u *Usecase) List(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Gateway], error) {
	return u.repository.ListPage(ctx, page)
}

// Get 查询单个 Gateway
func (u *Usecase) Get(ctx context.Context, gatewayID string) (*resource.Gateway, error) {
	return u.repository.Get(ctx, gatewayID)
}

// Create 创建 Gateway
func (u *Usecase) Create(ctx context.Context, spec resource.GatewaySpec) (*resource.Gateway, error) {
	if err := u.validateGateway(ctx, spec, ""); err != nil {
		return nil, err
	}

	id := uuid.NewString()
	gateway, err := u.repository.Create(ctx, id, spec)
	if err != nil {
		return nil, biz.DisplayNameConflict(err, "网关", spec.DisplayName)
	}
	return gateway, nil
}

// Update 使用配置版本乐观更新 Gateway
func (u *Usecase) Update(
	ctx context.Context,
	gatewayID string,
	version int64,
	spec resource.GatewaySpec,
) (*resource.Gateway, error) {
	current, err := u.repository.Get(ctx, gatewayID)
	if err != nil {
		return nil, err
	}
	if version != current.Generation {
		return nil, gatewayVersionConflict(current)
	}
	if err := u.validateGateway(ctx, spec, gatewayID); err != nil {
		return nil, err
	}
	updated, err := u.repository.Update(ctx, gatewayID, current.Generation, spec)
	if err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return nil, gatewayVersionConflict(current)
		}
		return nil, biz.DisplayNameConflict(err, "网关", spec.DisplayName)
	}
	return updated, nil
}

// Delete 删除 Gateway，仍有关联路由或策略时拒绝删除
func (u *Usecase) Delete(ctx context.Context, gatewayID string, version int64) error {
	current, err := u.repository.Get(ctx, gatewayID)
	if err != nil {
		return err
	}
	if version != current.Generation {
		return gatewayVersionConflict(current)
	}
	if err := biz.VisitPages(ctx, u.routes.ListPage, func(route resource.Route) (bool, error) {
		if slices.ContainsFunc(route.Spec.ParentRefs, func(parentRef resource.ParentRef) bool {
			return parentRef.Name == gatewayID
		}) {
			return true, biz.NewUserError(fmt.Sprintf("网关 %q 仍有关联路由", current.Spec.DisplayName))
		}
		return false, nil
	}); err != nil {
		return err
	}
	usage, err := u.policyUsage.Find(ctx, resource.PolicyTargetRef{Kind: resource.KindGateway, Name: gatewayID})
	if err != nil {
		return err
	}
	if usage != nil {
		return biz.NewUserError(fmt.Sprintf("网关 %q 仍被策略 %q 应用", current.Spec.DisplayName, usage.DisplayName))
	}
	if err := u.repository.Delete(ctx, gatewayID, current.Generation); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return gatewayVersionConflict(current)
		}
		return err
	}
	return nil
}

func gatewayVersionConflict(gateway *resource.Gateway) error {
	return biz.NewVersionConflictError(
		gateway.Name,
		fmt.Sprintf("网关 %q 已被其他用户修改，请刷新后重试", gateway.Spec.DisplayName),
	)
}

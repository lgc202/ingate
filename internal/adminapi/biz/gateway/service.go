// Package gateway 处理 Gateway 的业务规则和资源协作
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

// Repository 定义 Gateway 管理需要的持久化能力
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

// Service 协调 Gateway 的校验、引用约束和持久化
type Service struct {
	repository   Repository
	routes       RouteRepository
	certificates CertificateRepository
	policyUsage  *biz.PolicyUsageFinder
}

// NewService 创建 Gateway 业务服务
func NewService(
	repository Repository,
	routes RouteRepository,
	certificates CertificateRepository,
	policyUsage *biz.PolicyUsageFinder,
) *Service {
	return &Service{
		repository:   repository,
		routes:       routes,
		certificates: certificates,
		policyUsage:  policyUsage,
	}
}

// List 查询 Gateway 列表
func (s *Service) List(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Gateway], error) {
	return s.repository.ListPage(ctx, page)
}

// Get 查询单个 Gateway
func (s *Service) Get(ctx context.Context, gatewayID string) (*resource.Gateway, error) {
	return s.repository.Get(ctx, gatewayID)
}

// Create 创建 Gateway
func (s *Service) Create(ctx context.Context, spec resource.GatewaySpec) (*resource.Gateway, error) {
	if err := s.validateDisplayName(ctx, "", spec.DisplayName); err != nil {
		return nil, err
	}
	if err := s.validateGateway(ctx, spec, ""); err != nil {
		return nil, err
	}

	id := uuid.NewString()
	return s.repository.Create(ctx, id, spec)
}

// Update 使用配置版本乐观更新 Gateway
func (s *Service) Update(
	ctx context.Context,
	gatewayID string,
	version int64,
	spec resource.GatewaySpec,
) (*resource.Gateway, error) {
	current, err := s.repository.Get(ctx, gatewayID)
	if err != nil {
		return nil, err
	}
	if version != current.Generation {
		return nil, gatewayVersionConflict(current)
	}
	if spec.DisplayName != current.Spec.DisplayName {
		if err := s.validateDisplayName(ctx, gatewayID, spec.DisplayName); err != nil {
			return nil, err
		}
	}
	if err := s.validateGateway(ctx, spec, gatewayID); err != nil {
		return nil, err
	}
	updated, err := s.repository.Update(ctx, gatewayID, current.Generation, spec)
	if err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return nil, gatewayVersionConflict(current)
		}
		return nil, err
	}
	return updated, nil
}

// Delete 删除 Gateway，仍有关联路由或策略时拒绝删除
func (s *Service) Delete(ctx context.Context, gatewayID string, version int64) error {
	current, err := s.repository.Get(ctx, gatewayID)
	if err != nil {
		return err
	}
	if version != current.Generation {
		return gatewayVersionConflict(current)
	}
	if err := biz.VisitPages(ctx, s.routes.ListPage, func(route resource.Route) (bool, error) {
		if slices.Contains(route.Spec.GatewayRefs, gatewayID) {
			return true, biz.NewUserError(fmt.Sprintf("网关 %q 仍有关联路由", current.Spec.DisplayName))
		}
		return false, nil
	}); err != nil {
		return err
	}
	usage, err := s.policyUsage.Find(ctx, resource.PolicyTargetRef{Kind: resource.KindGateway, Name: gatewayID})
	if err != nil {
		return err
	}
	if usage != nil {
		return biz.NewUserError(fmt.Sprintf("网关 %q 仍被策略 %q 应用", current.Spec.DisplayName, usage.DisplayName))
	}
	if err := s.repository.Delete(ctx, gatewayID, current.Generation); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return gatewayVersionConflict(current)
		}
		return err
	}
	return nil
}

func (s *Service) validateDisplayName(ctx context.Context, gatewayID, displayName string) error {
	return biz.VisitPages(ctx, s.repository.ListPage, func(gateway resource.Gateway) (bool, error) {
		if gateway.Name != gatewayID && gateway.Spec.DisplayName == displayName {
			return true, biz.NewUserError(fmt.Sprintf("网关名称 %q 已存在", displayName))
		}
		return false, nil
	})
}

func gatewayVersionConflict(gateway *resource.Gateway) error {
	return biz.NewVersionConflictError(
		gateway.Name,
		fmt.Sprintf("网关 %q 已被其他用户修改，请刷新后重试", gateway.Spec.DisplayName),
	)
}

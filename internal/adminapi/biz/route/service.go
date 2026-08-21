// Package route 处理 Route 的业务规则和资源协作
package route

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Repository 定义 Route 管理需要的持久化能力
type Repository interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Route], error)
	Get(ctx context.Context, routeID string) (*resource.Route, error)
	Create(ctx context.Context, routeID string, spec resource.RouteSpec) (*resource.Route, error)
	Update(ctx context.Context, routeID string, generation int64, spec resource.RouteSpec) (*resource.Route, error)
	Delete(ctx context.Context, routeID string, generation int64) error
}

// GatewayRepository 定义 Route 校验 Gateway 引用时需要的查询能力
type GatewayRepository interface {
	Get(ctx context.Context, gatewayID string) (*resource.Gateway, error)
}

// UpstreamRepository 定义 Route 校验转发目标时需要的查询能力
type UpstreamRepository interface {
	Get(ctx context.Context, upstreamID string) (*resource.Upstream, error)
}

// CallerRepository 定义 Route 删除时需要的 Caller 授权查询能力
type CallerRepository interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Caller], error)
}

// Service 协调 Route 的校验、引用约束和持久化
type Service struct {
	repository  Repository
	gateways    GatewayRepository
	upstreams   UpstreamRepository
	callers     CallerRepository
	policyUsage *biz.PolicyUsageFinder
}

// NewService 创建 Route 业务服务
func NewService(
	repository Repository,
	gateways GatewayRepository,
	upstreams UpstreamRepository,
	callers CallerRepository,
	policyUsage *biz.PolicyUsageFinder,
) *Service {
	return &Service{
		repository:  repository,
		gateways:    gateways,
		upstreams:   upstreams,
		callers:     callers,
		policyUsage: policyUsage,
	}
}

// List 查询 Route 列表
func (s *Service) List(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Route], error) {
	return s.repository.ListPage(ctx, page)
}

// Get 查询单个 Route
func (s *Service) Get(ctx context.Context, routeID string) (*resource.Route, error) {
	return s.repository.Get(ctx, routeID)
}

// Create 创建 Route
func (s *Service) Create(ctx context.Context, spec resource.RouteSpec) (*resource.Route, error) {
	if err := s.ensureDisplayNameAvailable(ctx, "", spec.DisplayName); err != nil {
		return nil, err
	}
	if err := s.validateReferences(ctx, spec); err != nil {
		return nil, err
	}

	return s.repository.Create(ctx, uuid.NewString(), spec)
}

// Update 使用配置版本乐观更新 Route
func (s *Service) Update(
	ctx context.Context,
	routeID string,
	version int64,
	spec resource.RouteSpec,
) (*resource.Route, error) {
	current, err := s.repository.Get(ctx, routeID)
	if err != nil {
		return nil, err
	}
	if version != current.Generation {
		return nil, routeVersionConflict(current)
	}
	if spec.DisplayName != current.Spec.DisplayName {
		if err := s.ensureDisplayNameAvailable(ctx, routeID, spec.DisplayName); err != nil {
			return nil, err
		}
	}
	if err := s.validateReferences(ctx, spec); err != nil {
		return nil, err
	}

	updated, err := s.repository.Update(ctx, routeID, current.Generation, spec)
	if err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return nil, routeVersionConflict(current)
		}
		return nil, err
	}
	return updated, nil
}

// Delete 删除 Route，仍被策略应用时拒绝删除
func (s *Service) Delete(ctx context.Context, routeID string, version int64) error {
	current, err := s.repository.Get(ctx, routeID)
	if err != nil {
		return err
	}
	if version != current.Generation {
		return routeVersionConflict(current)
	}
	if err := s.ensureNotReferenced(ctx, current); err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, routeID, current.Generation); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return routeVersionConflict(current)
		}
		return err
	}
	return nil
}

func (s *Service) ensureDisplayNameAvailable(ctx context.Context, routeID, displayName string) error {
	return biz.VisitPages(ctx, s.repository.ListPage, func(route resource.Route) (bool, error) {
		if route.Name != routeID && route.Spec.DisplayName == displayName {
			return true, biz.NewRuleViolation(fmt.Sprintf("路由名称 %q 已存在", displayName))
		}
		return false, nil
	})
}

func routeVersionConflict(route *resource.Route) error {
	return biz.NewVersionConflict(
		route.Name,
		fmt.Sprintf("路由 %q 已被其他用户修改，请刷新后重试", route.Spec.DisplayName),
	)
}

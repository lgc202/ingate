// Package route 实现 Route 管理用例
package route

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Repository 定义 Route 用例需要的持久化能力
type Repository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.Route], error)
	Get(context.Context, string) (*resource.Route, error)
	Create(context.Context, string, resource.RouteSpec) (*resource.Route, error)
	Update(context.Context, string, int64, resource.RouteSpec) (*resource.Route, error)
	Delete(context.Context, string, int64) error
}

// GatewayRepository 定义 Route 校验 Gateway 引用时需要的查询能力
type GatewayRepository interface {
	Get(context.Context, string) (*resource.Gateway, error)
}

// UpstreamRepository 定义 Route 校验转发目标时需要的查询能力
type UpstreamRepository interface {
	Get(context.Context, string) (*resource.Upstream, error)
}

// Usecase 承载 Route 管理用例
type Usecase struct {
	repository  Repository
	gateways    GatewayRepository
	upstreams   UpstreamRepository
	policyUsage *biz.PolicyUsageFinder
}

// NewUsecase 创建路由管理用例
func NewUsecase(
	repository Repository,
	gateways GatewayRepository,
	upstreams UpstreamRepository,
	policyUsage *biz.PolicyUsageFinder,
) *Usecase {
	return &Usecase{
		repository:  repository,
		gateways:    gateways,
		upstreams:   upstreams,
		policyUsage: policyUsage,
	}
}

// List 查询 Route 列表
func (u *Usecase) List(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Route], error) {
	return u.repository.ListPage(ctx, page)
}

// Get 查询单个 Route
func (u *Usecase) Get(ctx context.Context, routeID string) (*resource.Route, error) {
	return u.repository.Get(ctx, routeID)
}

// Create 创建 Route
func (u *Usecase) Create(ctx context.Context, spec resource.RouteSpec) (*resource.Route, error) {
	if err := u.validateReferences(ctx, spec); err != nil {
		return nil, err
	}

	id := uuid.NewString()
	route, err := u.repository.Create(ctx, id, spec)
	if err != nil {
		return nil, biz.DisplayNameConflict(err, "路由", spec.DisplayName)
	}
	return route, nil
}

// Update 使用配置版本乐观更新 Route
func (u *Usecase) Update(
	ctx context.Context,
	routeID string,
	version int64,
	spec resource.RouteSpec,
) (*resource.Route, error) {
	current, err := u.repository.Get(ctx, routeID)
	if err != nil {
		return nil, err
	}
	if version != current.Generation {
		return nil, routeVersionConflict(current)
	}
	if err := u.validateReferences(ctx, spec); err != nil {
		return nil, err
	}

	updated, err := u.repository.Update(ctx, routeID, current.Generation, spec)
	if err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return nil, routeVersionConflict(current)
		}
		return nil, biz.DisplayNameConflict(err, "路由", spec.DisplayName)
	}
	return updated, nil
}

// Delete 删除 Route，仍被策略应用时拒绝删除
func (u *Usecase) Delete(ctx context.Context, routeID string, version int64) error {
	current, err := u.repository.Get(ctx, routeID)
	if err != nil {
		return err
	}
	if version != current.Generation {
		return routeVersionConflict(current)
	}
	usage, err := u.policyUsage.Find(ctx, resource.PolicyTargetRef{Kind: resource.KindRoute, Name: routeID})
	if err != nil {
		return err
	}
	if usage != nil {
		return biz.NewUserError(fmt.Sprintf("路由 %q 仍被策略 %q 应用", current.Spec.DisplayName, usage.DisplayName))
	}
	if err := u.repository.Delete(ctx, routeID, current.Generation); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return routeVersionConflict(current)
		}
		return err
	}
	return nil
}

func routeVersionConflict(route *resource.Route) error {
	return biz.NewVersionConflictError(
		route.Name,
		fmt.Sprintf("路由 %q 已被其他用户修改，请刷新后重试", route.Spec.DisplayName),
	)
}

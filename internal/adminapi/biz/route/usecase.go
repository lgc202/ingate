// Package route 实现 Route 管理用例
package route

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Repository 定义 Route 用例需要的持久化能力
type Repository interface {
	List(context.Context) ([]resource.Route, error)
	Get(context.Context, string) (*resource.Route, error)
	Create(context.Context, string, resource.RouteSpec) error
	Update(context.Context, string, int64, resource.RouteSpec) error
	Delete(context.Context, string) error
}

// GatewayRepository 定义 Route 校验父级引用时需要的查询能力
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
func (u *Usecase) List(ctx context.Context) ([]resource.Route, error) {
	return u.repository.List(ctx)
}

// Get 查询单个 Route
func (u *Usecase) Get(ctx context.Context, routeID string) (*resource.Route, error) {
	return u.repository.Get(ctx, routeID)
}

// Create 创建 Route
func (u *Usecase) Create(ctx context.Context, spec resource.RouteSpec) (string, error) {
	if err := u.validateNameUnique(ctx, spec.DisplayName, ""); err != nil {
		return "", err
	}
	if err := u.validateReferences(ctx, spec); err != nil {
		return "", err
	}
	id := uuid.NewString()
	if err := u.repository.Create(ctx, id, spec); err != nil {
		return "", err
	}
	return id, nil
}

// Update 更新 Route 配置
func (u *Usecase) Update(ctx context.Context, routeID, version string, submitted resource.RouteSpec) error {
	current, err := u.repository.Get(ctx, routeID)
	if err != nil {
		return err
	}
	if version != strconv.FormatInt(current.Generation, 10) {
		return biz.NewUserError(fmt.Sprintf("路由 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
	}
	if !supportsConsoleEditing(current.Spec) {
		return biz.NewUserError("该路由包含控制台暂不支持的配置，请通过声明式 API 修改")
	}
	if err := u.validateNameUnique(ctx, submitted.DisplayName, routeID); err != nil {
		return err
	}
	if err := u.validateReferences(ctx, submitted); err != nil {
		return err
	}

	if err := u.repository.Update(ctx, routeID, current.Generation, submitted); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return biz.NewUserError(fmt.Sprintf("路由 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// SetEnabled 更新 Route 启停状态
func (u *Usecase) SetEnabled(ctx context.Context, routeID string, enabled bool) error {
	current, err := u.repository.Get(ctx, routeID)
	if err != nil {
		return err
	}
	next := current.Spec
	next.Enabled = enabled
	if err := u.repository.Update(ctx, routeID, current.Generation, next); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return biz.NewUserError(fmt.Sprintf("路由 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// Delete 删除 Route，仍被策略应用时拒绝删除
func (u *Usecase) Delete(ctx context.Context, routeID string) error {
	current, err := u.repository.Get(ctx, routeID)
	if err != nil {
		return err
	}
	usage, err := u.policyUsage.Find(ctx, resource.PolicyTargetRef{Kind: resource.KindRoute, Name: routeID})
	if err != nil {
		return err
	}
	if usage != nil {
		return biz.NewUserError(fmt.Sprintf("路由 %q 仍被策略 %q 应用", current.Spec.DisplayName, usage.DisplayName))
	}
	return u.repository.Delete(ctx, routeID)
}

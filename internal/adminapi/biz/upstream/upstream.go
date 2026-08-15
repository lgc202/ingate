// Package upstream 实现 Upstream 管理用例
package upstream

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Repository 定义 Upstream 用例需要的持久化能力
type Repository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.Upstream], error)
	Get(context.Context, string) (*resource.Upstream, error)
	Create(context.Context, string, resource.UpstreamSpec) (*resource.Upstream, error)
	Update(context.Context, string, int64, resource.UpstreamSpec) (*resource.Upstream, error)
	Delete(context.Context, string, int64) error
}

// RouteRepository 定义 Upstream 变更时需要的 Route 查询能力
type RouteRepository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.Route], error)
}

// Usecase 承载 Upstream 管理用例
type Usecase struct {
	repository Repository
	routes     RouteRepository
}

// NewUsecase 创建服务管理用例
func NewUsecase(repository Repository, routes RouteRepository) *Usecase {
	return &Usecase{repository: repository, routes: routes}
}

// List 查询 Upstream 列表
func (u *Usecase) List(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.Upstream], error) {
	return u.repository.ListPage(ctx, page)
}

// Get 查询单个 Upstream
func (u *Usecase) Get(ctx context.Context, upstreamID string) (*resource.Upstream, error) {
	return u.repository.Get(ctx, upstreamID)
}

// Create 创建 Upstream
func (u *Usecase) Create(ctx context.Context, spec resource.UpstreamSpec) (*resource.Upstream, error) {
	if err := u.validateDisplayName(ctx, "", spec.DisplayName); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	return u.repository.Create(ctx, id, spec)
}

// Update 使用配置版本乐观更新 Upstream
func (u *Usecase) Update(
	ctx context.Context,
	upstreamID string,
	version int64,
	spec resource.UpstreamSpec,
) (*resource.Upstream, error) {
	current, err := u.repository.Get(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	if version != current.Generation {
		return nil, upstreamVersionConflict(current)
	}
	if spec.DisplayName != current.Spec.DisplayName {
		if err := u.validateDisplayName(ctx, upstreamID, spec.DisplayName); err != nil {
			return nil, err
		}
	}
	updated, err := u.repository.Update(ctx, upstreamID, current.Generation, spec)
	if err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return nil, upstreamVersionConflict(current)
		}
		return nil, err
	}
	return updated, nil
}

// Delete 删除 Upstream，仍有关联路由时拒绝删除
func (u *Usecase) Delete(ctx context.Context, upstreamID string, version int64) error {
	current, err := u.repository.Get(ctx, upstreamID)
	if err != nil {
		return err
	}
	if version != current.Generation {
		return upstreamVersionConflict(current)
	}
	if err := biz.VisitPages(ctx, u.routes.ListPage, func(route resource.Route) (bool, error) {
		for _, ref := range route.Spec.UpstreamRefs {
			if ref.Name == upstreamID {
				return true, biz.NewUserError(fmt.Sprintf("服务 %q 仍被路由 %q 引用", current.Spec.DisplayName, routeDisplayName(route)))
			}
		}
		return false, nil
	}); err != nil {
		return err
	}
	if err := u.repository.Delete(ctx, upstreamID, current.Generation); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return upstreamVersionConflict(current)
		}
		return err
	}
	return nil
}

func (u *Usecase) validateDisplayName(ctx context.Context, upstreamID, displayName string) error {
	return biz.VisitPages(ctx, u.repository.ListPage, func(upstream resource.Upstream) (bool, error) {
		if upstream.Name != upstreamID && upstream.Spec.DisplayName == displayName {
			return true, biz.NewUserError(fmt.Sprintf("服务名称 %q 已存在", displayName))
		}
		return false, nil
	})
}

func routeDisplayName(route resource.Route) string {
	if route.Spec.DisplayName != "" {
		return route.Spec.DisplayName
	}
	return route.Name
}

func upstreamVersionConflict(upstream *resource.Upstream) error {
	return biz.NewVersionConflictError(
		upstream.Name,
		fmt.Sprintf("服务 %q 已被其他用户修改，请刷新后重试", upstream.Spec.DisplayName),
	)
}

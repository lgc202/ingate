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
	id := uuid.NewString()
	upstream, err := u.repository.Create(ctx, id, spec)
	if err != nil {
		return nil, biz.DisplayNameConflict(err, "服务", spec.DisplayName)
	}
	return upstream, nil
}

// Update 使用配置版本乐观更新 Upstream
func (u *Usecase) Update(
	ctx context.Context,
	upstreamID string,
	version int64,
	spec resource.UpstreamSpec,
	apiKey *string,
) (*resource.Upstream, error) {
	current, err := u.repository.Get(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	if version != current.Generation {
		return nil, upstreamVersionConflict(current)
	}
	if spec.Model != nil && apiKey == nil && current.Spec.Model != nil {
		spec.Model.APIKey = current.Spec.Model.APIKey
	}
	if err := validateModelAPIKey(spec); err != nil {
		return nil, err
	}
	if err := u.validateRouteCompatibility(ctx, upstreamID, &resource.Upstream{Spec: spec}); err != nil {
		return nil, err
	}

	updated, err := u.repository.Update(ctx, upstreamID, current.Generation, spec)
	if err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return nil, upstreamVersionConflict(current)
		}
		return nil, biz.DisplayNameConflict(err, "服务", spec.DisplayName)
	}
	return updated, nil
}

func (u *Usecase) validateRouteCompatibility(
	ctx context.Context,
	upstreamID string,
	next *resource.Upstream,
) error {
	return biz.VisitPages(ctx, u.routes.ListPage, func(route resource.Route) (bool, error) {
		for _, ref := range route.Spec.UpstreamRefs {
			if ref.Name == upstreamID && next.Spec.Type == resource.UpstreamTypeModel {
				return true, biz.NewUserError(fmt.Sprintf("服务仍被普通路由 %q 引用，不能改为模型服务", routeDisplayName(route)))
			}
		}
		if route.Spec.ModelRouting == nil {
			return false, nil
		}
		for _, model := range route.Spec.ModelRouting.Models {
			if model.UpstreamRef != upstreamID {
				continue
			}
			if !ValidModel(next) {
				return true, biz.NewUserError(fmt.Sprintf("服务仍被模型路由 %q 引用，必须保持为有效的模型服务", routeDisplayName(route)))
			}
			upstreamModel := model.UpstreamModel
			if upstreamModel == "" {
				upstreamModel = model.Model
			}
			if !HasModel(next.Spec.Model, upstreamModel) {
				return true, biz.NewUserError(fmt.Sprintf("服务仍被模型路由 %q 的公开模型 %q 引用，不能删除厂商模型 %q", routeDisplayName(route), model.Model, upstreamModel))
			}
		}
		return false, nil
	})
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
		if route.Spec.ModelRouting != nil {
			for _, model := range route.Spec.ModelRouting.Models {
				if model.UpstreamRef == upstreamID {
					return true, biz.NewUserError(fmt.Sprintf("服务 %q 仍被路由 %q 的公开模型 %q 引用", current.Spec.DisplayName, routeDisplayName(route), model.Model))
				}
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

func routeDisplayName(route resource.Route) string {
	if route.Spec.DisplayName != "" {
		return route.Spec.DisplayName
	}
	return route.Name
}

func validateModelAPIKey(spec resource.UpstreamSpec) error {
	if spec.Model == nil || spec.Model.APIKey == "" {
		return nil
	}
	if spec.TLS == nil {
		return biz.NewUserError("服务已配置 API Key，关闭 HTTPS 前请先清空 API Key")
	}
	return nil
}

func upstreamVersionConflict(upstream *resource.Upstream) error {
	return biz.NewVersionConflictError(
		upstream.Name,
		fmt.Sprintf("服务 %q 已被其他用户修改，请刷新后重试", upstream.Spec.DisplayName),
	)
}

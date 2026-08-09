// Package upstream 实现 Upstream 管理用例
package upstream

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Repository 定义 Upstream 用例需要的持久化能力
type Repository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.Upstream], error)
	Get(context.Context, string) (*resource.Upstream, error)
	Create(context.Context, string, resource.UpstreamSpec) error
	Update(context.Context, string, int64, resource.UpstreamSpec) error
	Delete(context.Context, string) error
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
	upstream, err := u.repository.Get(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	return upstream, nil
}

// Create 创建 Upstream
func (u *Usecase) Create(ctx context.Context, spec resource.UpstreamSpec) (string, error) {
	id := uuid.NewString()
	if err := u.repository.Create(ctx, id, spec); err != nil {
		return "", biz.DisplayNameConflict(err, "服务", spec.DisplayName)
	}
	return id, nil
}

// Update 更新 Upstream
func (u *Usecase) Update(
	ctx context.Context,
	upstreamID string,
	version string,
	spec resource.UpstreamSpec,
	removeAPIKey bool,
) error {
	current, err := u.repository.Get(ctx, upstreamID)
	if err != nil {
		return err
	}
	if version != strconv.FormatInt(current.Generation, 10) {
		return biz.NewUserError(fmt.Sprintf("服务 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
	}
	currentAuthentication := current.Spec.Authentication
	if removeAPIKey {
		spec.Authentication = nil
	} else if spec.Authentication == nil {
		spec.Authentication = currentAuthentication
	}
	next := &resource.Upstream{Spec: spec}
	if err := validateAuthentication(next); err != nil {
		return err
	}
	if err := u.validateRouteCompatibility(ctx, upstreamID, next); err != nil {
		return err
	}
	if err := u.repository.Update(ctx, upstreamID, current.Generation, spec); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return biz.NewUserError(fmt.Sprintf("服务 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return biz.DisplayNameConflict(err, "服务", spec.DisplayName)
	}
	return nil
}

func (u *Usecase) validateRouteCompatibility(
	ctx context.Context,
	upstreamID string,
	next *resource.Upstream,
) error {
	return biz.VisitPages(ctx, u.routes.ListPage, func(route resource.Route) (bool, error) {
		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.UpstreamRefs {
				if ref.Name == upstreamID && (next.Spec.Type == resource.UpstreamTypeModel || next.Spec.Protocol != resource.UpstreamProtocolHTTP) {
					return true, biz.NewUserError(fmt.Sprintf("服务仍被普通路由 %q 引用，不能改为模型服务", routeDisplayName(route)))
				}
			}
			if rule.ModelRouting == nil {
				continue
			}
			for _, model := range rule.ModelRouting.Models {
				if model.UpstreamRef != upstreamID {
					continue
				}
				if !ValidModel(next) {
					return true, biz.NewUserError(fmt.Sprintf("服务仍被模型路由 %q 引用，必须保持为有效的大模型服务", routeDisplayName(route)))
				}
				upstreamModel := model.UpstreamModel
				if upstreamModel == "" {
					upstreamModel = model.Model
				}
				if !ModelEnabled(next.Spec.Model, upstreamModel) {
					return true, biz.NewUserError(fmt.Sprintf("服务仍被模型路由 %q 的公开模型 %q 引用，不能删除或禁用厂商模型 %q", routeDisplayName(route), model.Model, upstreamModel))
				}
			}
		}
		return false, nil
	})
}

// Delete 删除 Upstream，仍有关联路由时拒绝删除
func (u *Usecase) Delete(ctx context.Context, upstreamID string) error {
	if err := biz.VisitPages(ctx, u.routes.ListPage, func(route resource.Route) (bool, error) {
		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.UpstreamRefs {
				if ref.Name == upstreamID {
					return true, biz.NewUserError(fmt.Sprintf("服务 %q 仍被路由 %q 引用", upstreamID, routeDisplayName(route)))
				}
			}
			if rule.ModelRouting != nil {
				for _, model := range rule.ModelRouting.Models {
					if model.UpstreamRef == upstreamID {
						return true, biz.NewUserError(fmt.Sprintf("服务 %q 仍被路由 %q 的公开模型 %q 引用", upstreamID, routeDisplayName(route), model.Model))
					}
				}
			}
		}
		return false, nil
	}); err != nil {
		return err
	}
	return u.repository.Delete(ctx, upstreamID)
}

func routeDisplayName(route resource.Route) string {
	if route.Spec.DisplayName != "" {
		return route.Spec.DisplayName
	}
	return route.Name
}

func validateAuthentication(upstream *resource.Upstream) error {
	if upstream.Spec.Authentication == nil {
		return nil
	}
	if upstream.Spec.Type != resource.UpstreamTypeModel {
		return biz.NewUserError("服务已配置 API Key，必须保持为大模型服务")
	}
	if upstream.Spec.TLS == nil {
		return biz.NewUserError("服务已配置 API Key，关闭 HTTPS 前请先移除 API Key")
	}
	return nil
}

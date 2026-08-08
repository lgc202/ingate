// Package route 实现 Route 管理用例
package route

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// ProviderSet 提供 Route 管理用例
var ProviderSet = wire.NewSet(NewUsecase)

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
	if err := u.validateNameUnique(ctx, submitted.DisplayName, routeID); err != nil {
		return err
	}
	if err := u.validateReferences(ctx, submitted); err != nil {
		return err
	}

	next := mergeRouteSpec(current.Spec, submitted)
	if err := u.repository.Update(ctx, routeID, current.Generation, next); err != nil {
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

// mergeRouteSpec 保留声明式 API 已开放但控制台暂不编辑的规则字段
// 只有同名且仍保留对应能力的规则才继承 Add 和 RetryOn，删除或改名视为显式移除
func mergeRouteSpec(current, submitted resource.RouteSpec) resource.RouteSpec {
	currentByName := make(map[string]resource.RouteRule, len(current.Rules))
	for _, rule := range current.Rules {
		currentByName[rule.Name] = rule
	}

	rules := make([]resource.RouteRule, 0, len(submitted.Rules))
	for _, rule := range submitted.Rules {
		currentRule, exists := currentByName[rule.Name]
		if !exists {
			rules = append(rules, rule)
			continue
		}

		currentFilters := make(map[resource.RouteFilterType]resource.RouteFilter, len(currentRule.Filters))
		for _, filter := range currentRule.Filters {
			currentFilters[filter.Type] = filter
		}
		filters := make([]resource.RouteFilter, 0, len(rule.Filters))
		for _, filter := range rule.Filters {
			currentFilter := currentFilters[filter.Type]
			switch filter.Type {
			case resource.RouteFilterRequestHeaderModifier:
				if currentFilter.RequestHeaderModifier != nil && filter.RequestHeaderModifier != nil {
					modifier := *filter.RequestHeaderModifier
					modifier.Add = currentFilter.RequestHeaderModifier.Add
					filter.RequestHeaderModifier = &modifier
				}
			case resource.RouteFilterResponseHeaderModifier:
				if currentFilter.ResponseHeaderModifier != nil && filter.ResponseHeaderModifier != nil {
					modifier := *filter.ResponseHeaderModifier
					modifier.Add = currentFilter.ResponseHeaderModifier.Add
					filter.ResponseHeaderModifier = &modifier
				}
			}
			filters = append(filters, filter)
		}

		nextRule := rule
		nextRule.Filters = filters
		if currentRule.Retry != nil && rule.Retry != nil {
			retry := *rule.Retry
			retry.RetryOn = currentRule.Retry.RetryOn
			nextRule.Retry = &retry
		}
		rules = append(rules, nextRule)
	}

	current.DisplayName = submitted.DisplayName
	current.Enabled = submitted.Enabled
	current.ParentRefs = submitted.ParentRefs
	current.Hostnames = submitted.Hostnames
	current.Rules = rules
	return current
}

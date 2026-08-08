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
	upstreambiz "github.com/lgc202/ingate/internal/adminapi/biz/upstream"
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
	upstream    UpstreamRepository
	policyUsage *biz.PolicyUsageFinder
}

// NewUsecase 创建路由管理用例
func NewUsecase(
	repository Repository,
	gateways GatewayRepository,
	upstream UpstreamRepository,
	policyUsage *biz.PolicyUsageFinder,
) *Usecase {
	return &Usecase{
		repository:  repository,
		gateways:    gateways,
		upstream:    upstream,
		policyUsage: policyUsage,
	}
}

// List 查询 Route 列表
func (s *Usecase) List(ctx context.Context) ([]resource.Route, error) {
	routes, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	return routes, nil
}

// Get 查询单个 Route
func (s *Usecase) Get(ctx context.Context, routeID string) (*resource.Route, error) {
	route, err := s.repository.Get(ctx, routeID)
	if err != nil {
		return nil, err
	}
	return route, nil
}

// Create 创建 Route
func (s *Usecase) Create(ctx context.Context, spec resource.RouteSpec) (string, error) {
	if err := s.validateNameUnique(ctx, spec.DisplayName, ""); err != nil {
		return "", err
	}
	if err := s.validateReferences(ctx, spec); err != nil {
		return "", err
	}
	id := uuid.NewString()
	if err := s.repository.Create(ctx, id, spec); err != nil {
		return "", err
	}
	return id, nil
}

// Update 更新 Route
func (s *Usecase) Update(ctx context.Context, routeID, version string, spec resource.RouteSpec) error {
	current, err := s.repository.Get(ctx, routeID)
	if err != nil {
		return err
	}
	if version != strconv.FormatInt(current.Generation, 10) {
		return biz.NewUserError(fmt.Sprintf("路由 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
	}
	if err := s.validateNameUnique(ctx, spec.DisplayName, routeID); err != nil {
		return err
	}
	if err := s.validateReferences(ctx, spec); err != nil {
		return err
	}

	nextSpec := current.Spec
	s.applyRouteSpec(&nextSpec, spec)
	if err := s.repository.Update(ctx, routeID, current.Generation, nextSpec); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return biz.NewUserError(fmt.Sprintf("路由 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// SetEnabled 更新 Route 启停状态
func (s *Usecase) SetEnabled(ctx context.Context, routeID string, enabled bool) error {
	current, err := s.repository.Get(ctx, routeID)
	if err != nil {
		return err
	}
	spec := current.Spec
	spec.Enabled = enabled
	if err := s.repository.Update(ctx, routeID, current.Generation, spec); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return biz.NewUserError(fmt.Sprintf("路由 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// Delete 删除 Route，仍被策略应用时拒绝删除
func (s *Usecase) Delete(ctx context.Context, routeID string) error {
	current, err := s.repository.Get(ctx, routeID)
	if err != nil {
		return err
	}
	usage, err := s.policyUsage.Find(ctx, resource.PolicyTargetRef{Kind: resource.KindRoute, Name: routeID})
	if err != nil {
		return err
	}
	if usage != nil {
		return biz.NewUserError(fmt.Sprintf("路由 %q 仍被策略 %q 应用", current.Spec.DisplayName, usage.DisplayName))
	}
	return s.repository.Delete(ctx, routeID)
}

// applyRouteSpec 保留声明式 API 可配置但控制台暂未开放的规则字段
// 只有同名且仍保留对应能力的规则才继承 Add 和 RetryOn，删除或改名视为显式移除
func (s *Usecase) applyRouteSpec(current *resource.RouteSpec, submitted resource.RouteSpec) {
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

		next := rule
		next.Filters = filters
		if currentRule.Retry != nil && rule.Retry != nil {
			retry := *rule.Retry
			retry.RetryOn = currentRule.Retry.RetryOn
			next.Retry = &retry
		}
		rules = append(rules, next)
	}

	current.DisplayName = submitted.DisplayName
	current.Enabled = submitted.Enabled
	current.ParentRefs = submitted.ParentRefs
	current.Hostnames = submitted.Hostnames
	current.Rules = rules
}

func (s *Usecase) validateNameUnique(ctx context.Context, name, excludeID string) error {
	routes, err := s.repository.List(ctx)
	if err != nil {
		return err
	}
	for _, route := range routes {
		if route.Name == excludeID {
			continue
		}
		if route.Spec.DisplayName == name {
			return biz.NewUserError(fmt.Sprintf("路由名称 %q 已存在", name))
		}
	}
	return nil
}

func (s *Usecase) validateReferences(ctx context.Context, spec resource.RouteSpec) error {
	for _, parentRef := range spec.ParentRefs {
		if _, err := s.gateways.Get(ctx, parentRef.Name); err != nil {
			if errors.Is(err, biz.ErrResourceNotFound) {
				return biz.NewUserError(fmt.Sprintf("关联网关 %q 不存在", parentRef.Name))
			}
			return err
		}
	}
	upstreams := make(map[string]*resource.Upstream)
	for _, rule := range spec.Rules {
		for _, ref := range rule.UpstreamRefs {
			upstream, err := s.getUpstream(ctx, upstreams, ref.Name)
			if err != nil {
				if errors.Is(err, biz.ErrResourceNotFound) {
					return biz.NewUserError(fmt.Sprintf("关联服务 %q 不存在", ref.Name))
				}
				return err
			}
			if upstream.Spec.Type == resource.UpstreamTypeModel || upstream.Spec.Protocol != resource.UpstreamProtocolHTTP {
				return biz.NewUserError(fmt.Sprintf("模型服务 %q 只能用于模型路由", upstreamDisplayName(upstream)))
			}
		}
		if rule.ModelRouting == nil {
			continue
		}
		for _, model := range rule.ModelRouting.Models {
			upstream, err := s.getUpstream(ctx, upstreams, model.UpstreamRef)
			if err != nil {
				if errors.Is(err, biz.ErrResourceNotFound) {
					return biz.NewUserError(fmt.Sprintf("关联模型服务 %q 不存在", model.UpstreamRef))
				}
				return err
			}
			if !upstreambiz.ValidModel(upstream) {
				return biz.NewUserError(fmt.Sprintf("关联服务 %q 不是有效的大模型服务", upstreamDisplayName(upstream)))
			}
			upstreamModel := model.UpstreamModel
			if upstreamModel == "" {
				upstreamModel = model.Model
			}
			if !upstreambiz.ModelEnabled(upstream.Spec.Model, upstreamModel) {
				return biz.NewUserError(fmt.Sprintf("模型服务 %q 未启用厂商模型 %q", upstreamDisplayName(upstream), upstreamModel))
			}
		}
	}
	return nil
}

func (s *Usecase) getUpstream(
	ctx context.Context,
	upstreams map[string]*resource.Upstream,
	upstreamID string,
) (*resource.Upstream, error) {
	if upstream, ok := upstreams[upstreamID]; ok {
		return upstream, nil
	}
	upstream, err := s.upstream.Get(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	upstreams[upstreamID] = upstream
	return upstream, nil
}

func upstreamDisplayName(upstream *resource.Upstream) string {
	if upstream.Spec.DisplayName != "" {
		return upstream.Spec.DisplayName
	}
	return upstream.Name
}

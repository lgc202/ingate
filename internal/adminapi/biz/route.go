package biz

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/google/uuid"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// RouteUsecase 承载 Route 查询用例
type RouteUsecase struct {
	repository  RouteRepository
	gateways    GatewayRepository
	upstream    UpstreamRepository
	policyUsage *PolicyUsageFinder
}

// NewRouteUsecase 创建路由管理用例
func NewRouteUsecase(
	repository RouteRepository,
	gateways GatewayRepository,
	upstream UpstreamRepository,
	policyUsage *PolicyUsageFinder,
) *RouteUsecase {
	return &RouteUsecase{
		repository:  repository,
		gateways:    gateways,
		upstream:    upstream,
		policyUsage: policyUsage,
	}
}

// List 查询 Route 列表
func (s *RouteUsecase) List(ctx context.Context) ([]resource.Route, error) {
	routes, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	return routes.Items, nil
}

// Get 查询单个 Route
func (s *RouteUsecase) Get(ctx context.Context, routeID string) (*resource.Route, error) {
	route, err := s.repository.Get(ctx, routeID)
	if err != nil {
		return nil, err
	}
	return route, nil
}

// Create 创建 Route
func (s *RouteUsecase) Create(ctx context.Context, spec resource.RouteSpec) (string, error) {
	if err := s.validateNameUnique(ctx, spec.DisplayName, ""); err != nil {
		return "", err
	}
	if err := s.validateReferences(ctx, spec); err != nil {
		return "", err
	}
	route := &resource.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindRoute),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: uuid.NewString(),
		},
		Spec: spec,
	}
	_, err := s.repository.Create(ctx, route)
	if apierrors.IsAlreadyExists(err) {
		return "", NewUserError(fmt.Sprintf("路由 %q 已存在", route.Name))
	}
	if err != nil {
		return "", err
	}
	return route.Name, nil
}

// Update 更新 Route
func (s *RouteUsecase) Update(ctx context.Context, routeID, version string, spec resource.RouteSpec) error {
	if version == "" {
		return NewUserError("路由版本不能为空")
	}

	// version 对应配置 Generation；status 写入只改变 ResourceVersion，因此写冲突可以安全重试
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.repository.Get(ctx, routeID)
		if err != nil {
			return err
		}
		if version != strconv.FormatInt(current.Generation, 10) {
			return NewUserError(fmt.Sprintf("路由 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		if err := s.validateNameUnique(ctx, spec.DisplayName, routeID); err != nil {
			return err
		}
		if err := s.validateReferences(ctx, spec); err != nil {
			return err
		}

		next := current.DeepCopy()
		s.applyRouteSpec(&next.Spec, spec)
		_, err = s.repository.Update(ctx, next)
		return err
	})
}

// SetEnabled 更新 Route 启停状态
func (s *RouteUsecase) SetEnabled(ctx context.Context, routeID string, enabled bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.repository.Get(ctx, routeID)
		if err != nil {
			return err
		}

		next := current.DeepCopy()
		next.Spec.Enabled = enabled

		_, err = s.repository.Update(ctx, next)
		return err
	})
}

// Delete 删除 Route，仍被策略应用时拒绝删除
func (s *RouteUsecase) Delete(ctx context.Context, routeID string) error {
	current, err := s.repository.Get(ctx, routeID)
	if err != nil {
		return err
	}
	usage, err := s.policyUsage.Find(ctx, resource.PolicyTargetRef{Kind: resource.KindRoute, Name: routeID})
	if err != nil {
		return err
	}
	if usage != nil {
		return NewUserError(fmt.Sprintf("路由 %q 仍被策略 %q 应用", current.Spec.DisplayName, usage.DisplayName))
	}
	return s.repository.Delete(ctx, routeID)
}

// applyRouteSpec 保留声明式 API 可配置但控制台暂未开放的规则字段
// 只有同名且仍保留对应能力的规则才继承 Add 和 RetryOn，删除或改名视为显式移除
func (s *RouteUsecase) applyRouteSpec(current *resource.RouteSpec, submitted resource.RouteSpec) {
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

func (s *RouteUsecase) validateNameUnique(ctx context.Context, name, excludeID string) error {
	routes, err := s.repository.List(ctx)
	if err != nil {
		return err
	}
	for _, route := range routes.Items {
		if route.Name == excludeID {
			continue
		}
		if route.Spec.DisplayName == name {
			return NewUserError(fmt.Sprintf("路由名称 %q 已存在", name))
		}
	}
	return nil
}

func (s *RouteUsecase) validateReferences(ctx context.Context, spec resource.RouteSpec) error {
	for _, parentRef := range spec.ParentRefs {
		if _, err := s.gateways.Get(ctx, parentRef.Name); err != nil {
			if apierrors.IsNotFound(err) {
				return NewUserError(fmt.Sprintf("关联网关 %q 不存在", parentRef.Name))
			}
			return err
		}
	}
	upstreams := make(map[string]*resource.Upstream)
	for _, rule := range spec.Rules {
		for _, ref := range rule.UpstreamRefs {
			upstream, err := s.getUpstream(ctx, upstreams, ref.Name)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return NewUserError(fmt.Sprintf("关联服务 %q 不存在", ref.Name))
				}
				return err
			}
			if upstream.Spec.Type == resource.UpstreamTypeModel || upstream.Spec.Protocol != resource.UpstreamProtocolHTTP {
				return NewUserError(fmt.Sprintf("模型服务 %q 只能用于模型路由", upstreamDisplayName(upstream)))
			}
		}
		if rule.ModelRouting == nil {
			continue
		}
		for _, model := range rule.ModelRouting.Models {
			upstream, err := s.getUpstream(ctx, upstreams, model.UpstreamRef)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return NewUserError(fmt.Sprintf("关联模型服务 %q 不存在", model.UpstreamRef))
				}
				return err
			}
			if !validModelUpstream(upstream) {
				return NewUserError(fmt.Sprintf("关联服务 %q 不是有效的大模型服务", upstreamDisplayName(upstream)))
			}
			upstreamModel := model.UpstreamModel
			if upstreamModel == "" {
				upstreamModel = model.Model
			}
			if !enabledModel(upstream.Spec.Model, upstreamModel) {
				return NewUserError(fmt.Sprintf("模型服务 %q 未启用厂商模型 %q", upstreamDisplayName(upstream), upstreamModel))
			}
		}
	}
	return nil
}

func (s *RouteUsecase) getUpstream(
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

func validModelUpstream(upstream *resource.Upstream) bool {
	if upstream.Spec.Type != resource.UpstreamTypeModel || upstream.Spec.Model == nil {
		return false
	}
	providerProtocol, ok := upstream.Spec.Model.Provider.Protocol()
	if !ok ||
		upstream.Spec.Protocol != providerProtocol ||
		!validAPIBasePath(upstream.Spec.Model.APIBasePath) ||
		len(upstream.Spec.Model.Models) == 0 {
		return false
	}

	enabledModels := 0
	modelNames := make(map[string]struct{}, len(upstream.Spec.Model.Models))
	for _, model := range upstream.Spec.Model.Models {
		if model.Name == "" || strings.TrimSpace(model.Name) != model.Name ||
			model.DisplayName == "" || strings.TrimSpace(model.DisplayName) != model.DisplayName {
			return false
		}
		if _, exists := modelNames[model.Name]; exists {
			return false
		}
		modelNames[model.Name] = struct{}{}
		if model.Enabled {
			enabledModels++
		}
	}
	return enabledModels > 0
}

func enabledModel(modelSpec *resource.ModelSpec, name string) bool {
	for _, model := range modelSpec.Models {
		if model.Name == name {
			return model.Enabled
		}
	}
	return false
}

func validAPIBasePath(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || !strings.HasPrefix(value, "/") {
		return false
	}
	if value != "/" && strings.HasSuffix(value, "/") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != value {
		return false
	}
	return path.Clean(value) == value
}

func upstreamDisplayName(upstream *resource.Upstream) string {
	if upstream.Spec.DisplayName != "" {
		return upstream.Spec.DisplayName
	}
	return upstream.Name
}

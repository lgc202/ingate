package route

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	"github.com/lgc202/ingate/internal/adminapi/service/policytarget"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// Service 承载 Route 查询用例
type Service struct {
	store       *routestore.Store
	gateways    *gatewaystore.Store
	upstream    *upstreamstore.Store
	policyUsage *policytarget.UsageFinder
}

// New 创建 Route service
func New(
	store *routestore.Store,
	gateways *gatewaystore.Store,
	upstream *upstreamstore.Store,
	policyUsage *policytarget.UsageFinder,
) *Service {
	return &Service{
		store:       store,
		gateways:    gateways,
		upstream:    upstream,
		policyUsage: policyUsage,
	}
}

// List 查询 Route 列表
func (s *Service) List(ctx context.Context) (*ListResult, error) {
	routes, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ListResult{Routes: routes.Items}, nil
}

// Get 查询单个 Route
func (s *Service) Get(ctx context.Context, routeID string) (*RouteResult, error) {
	route, err := s.store.Get(ctx, routeID)
	if err != nil {
		return nil, err
	}
	return &RouteResult{Route: route}, nil
}

// Create 创建 Route
func (s *Service) Create(ctx context.Context, params CreateRouteParams) (string, error) {
	if err := s.validateNameUnique(ctx, params.Name, ""); err != nil {
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
		Spec: routeSpec(params),
	}
	if err := s.validateReferences(ctx, route); err != nil {
		return "", err
	}
	_, err := s.store.Create(ctx, route)
	if apierrors.IsAlreadyExists(err) {
		return "", xerrors.NewUserError(fmt.Sprintf("路由 %q 已存在", route.Name))
	}
	if err != nil {
		return "", err
	}
	return route.Name, nil
}

// Update 更新 Route
func (s *Service) Update(ctx context.Context, routeID string, params UpdateRouteParams) error {
	if params.Version == "" {
		return xerrors.NewUserError("路由版本不能为空")
	}
	current, err := s.store.Get(ctx, routeID)
	if err != nil {
		return err
	}
	if err := validateVersion(resource.ResourceRoutes, routeID, params.Version, current.ResourceVersion); err != nil {
		return err
	}
	if err := s.validateNameUnique(ctx, params.Name, routeID); err != nil {
		return err
	}

	spec := routeSpec(params.CreateRouteParams)
	next := current.DeepCopy()
	next.Spec = spec
	if err := s.validateReferences(ctx, next); err != nil {
		return err
	}
	_, err = s.store.Update(ctx, next)
	return err
}

// SetEnabled 更新 Route 启停状态
func (s *Service) SetEnabled(ctx context.Context, routeID string, enabled bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.store.Get(ctx, routeID)
		if err != nil {
			return err
		}

		next := current.DeepCopy()
		next.Spec.Enabled = enabled

		_, err = s.store.Update(ctx, next)
		return err
	})
}

// Delete 删除 Route，仍被策略应用时拒绝删除
func (s *Service) Delete(ctx context.Context, routeID string) error {
	current, err := s.store.Get(ctx, routeID)
	if err != nil {
		return err
	}
	usage, err := s.policyUsage.Find(ctx, resource.PolicyTargetRef{Kind: resource.KindRoute, Name: routeID})
	if err != nil {
		return err
	}
	if usage != nil {
		return xerrors.NewUserError(fmt.Sprintf("路由 %q 仍被策略 %q 应用", current.Spec.DisplayName, usage.DisplayName))
	}
	return s.store.Delete(ctx, routeID)
}

func (s *Service) validateNameUnique(ctx context.Context, name, excludeID string) error {
	routes, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	for _, route := range routes.Items {
		if route.Name == excludeID {
			continue
		}
		if route.Spec.DisplayName == name {
			return xerrors.NewUserError(fmt.Sprintf("路由名称 %q 已存在", name))
		}
	}
	return nil
}

func (s *Service) validateReferences(ctx context.Context, route *resource.Route) error {
	for _, parentRef := range route.Spec.ParentRefs {
		if _, err := s.gateways.Get(ctx, parentRef.Name); err != nil {
			if apierrors.IsNotFound(err) {
				return xerrors.NewUserError(fmt.Sprintf("关联网关 %q 不存在", parentRef.Name))
			}
			return err
		}
	}
	for _, rule := range route.Spec.Rules {
		for _, ref := range rule.UpstreamRefs {
			upstream, err := s.upstream.Get(ctx, ref.Name)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return xerrors.NewUserError(fmt.Sprintf("关联服务 %q 不存在", ref.Name))
				}
				return err
			}
			if upstream.Spec.Protocol == resource.UpstreamProtocolOpenAI {
				return xerrors.NewUserError(fmt.Sprintf("模型服务 %q 只能用于模型路由", upstream.Spec.DisplayName))
			}
		}
		if rule.ModelRouting == nil {
			continue
		}
		upstream, err := s.upstream.Get(ctx, rule.ModelRouting.UpstreamRef)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return xerrors.NewUserError(fmt.Sprintf("关联模型服务 %q 不存在", rule.ModelRouting.UpstreamRef))
			}
			return err
		}
		if upstream.Spec.Type != resource.UpstreamTypeModel || upstream.Spec.Protocol != resource.UpstreamProtocolOpenAI {
			return xerrors.NewUserError(fmt.Sprintf("关联服务 %q 不是 OpenAI 兼容模型服务", upstream.Spec.DisplayName))
		}
	}
	return nil
}

func routeSpec(params CreateRouteParams) resource.RouteSpec {
	return resource.RouteSpec{
		DisplayName: params.Name,
		Enabled:     params.Enabled,
		ParentRefs:  parentRefs(params.GatewayIDs),
		Hostnames:   params.Hostnames,
		Rules:       routeRules(params.Rules),
	}
}

func routeRules(params []RouteRuleParams) []resource.RouteRule {
	rules := make([]resource.RouteRule, 0, len(params))
	for _, item := range params {
		rule := resource.RouteRule{
			Name:         item.Name,
			PathPrefix:   item.PathPrefix,
			Methods:      item.Methods,
			Headers:      headerMatches(item.Headers),
			UpstreamRefs: upstreamRefs(item.Targets),
			ModelRouting: modelRouting(item.ModelRouting),
		}
		if item.RequestHeaderModifier != nil {
			rule.Filters = append(rule.Filters, resource.RouteFilter{
				Type:                  resource.RouteFilterRequestHeaderModifier,
				RequestHeaderModifier: headerModifier(item.RequestHeaderModifier),
			})
		}
		if item.ResponseHeaderModifier != nil {
			rule.Filters = append(rule.Filters, resource.RouteFilter{
				Type:                   resource.RouteFilterResponseHeaderModifier,
				ResponseHeaderModifier: headerModifier(item.ResponseHeaderModifier),
			})
		}
		if item.Timeout != nil {
			rule.Timeout = &resource.RouteTimeout{RequestMillis: item.Timeout.RequestMillis}
		}
		if item.Retry != nil {
			rule.Retry = &resource.RouteRetry{
				Attempts:            item.Retry.Attempts,
				PerTryTimeoutMillis: item.Retry.PerTryTimeoutMillis,
			}
		}
		rules = append(rules, rule)
	}
	return rules
}

func modelRouting(params *ModelRoutingParams) *resource.ModelRouting {
	if params == nil {
		return nil
	}
	return &resource.ModelRouting{
		UpstreamRef: params.UpstreamID,
		Models: lo.Map(params.Models, func(model ModelRouteParams, _ int) resource.ModelRoute {
			return resource.ModelRoute{
				Model:         model.Model,
				UpstreamModel: model.UpstreamModel,
			}
		}),
	}
}

func parentRefs(gatewayIDs []string) []resource.ParentRef {
	return lo.Map(gatewayIDs, func(gatewayID string, _ int) resource.ParentRef {
		return resource.ParentRef{Name: gatewayID}
	})
}

func upstreamRefs(targets []TargetParams) []resource.UpstreamRef {
	return lo.Map(targets, func(target TargetParams, _ int) resource.UpstreamRef {
		return resource.UpstreamRef{
			Name:   target.UpstreamID,
			Weight: target.Weight,
		}
	})
}

func headerMatches(headers []HeaderMatchParams) []resource.HeaderMatch {
	return lo.Map(headers, func(header HeaderMatchParams, _ int) resource.HeaderMatch {
		return resource.HeaderMatch{
			Name:  header.Name,
			Value: header.Value,
		}
	})
}

func headerModifier(params *HeaderModifierParams) *resource.HeaderModifier {
	return &resource.HeaderModifier{
		Set: lo.Map(params.Set, func(header HeaderValueParams, _ int) resource.HeaderValue {
			return resource.HeaderValue{
				Name:  header.Name,
				Value: header.Value,
			}
		}),
		Remove: params.Remove,
	}
}

func validateVersion(resourceName resource.ResourceName, name, submittedVersion, currentVersion string) error {
	if submittedVersion == currentVersion {
		return nil
	}
	return xerrors.NewUserError(fmt.Sprintf("%s %q 已被更新，请刷新后重试", resourceName, name))
}

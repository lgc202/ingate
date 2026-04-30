package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	resourcestore "github.com/lgc202/ingate/internal/adminapi/store/resource"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Kind 表示 admin-api 暴露的声明式资源集合
type Kind = resourcestore.Kind

const (
	// KindAIRoutes 表示 AI 路由资源
	KindAIRoutes = resourcestore.KindAIRoutes
	// KindAIProviders 表示 AI 供应商资源
	KindAIProviders = resourcestore.KindAIProviders
	// KindAIModels 表示 AI 模型资源
	KindAIModels = resourcestore.KindAIModels
	// KindAIPolicies 表示 AI 策略资源
	KindAIPolicies = resourcestore.KindAIPolicies
	// KindPlugins 表示插件资源
	KindPlugins = resourcestore.KindPlugins
	// KindPluginBindings 表示插件绑定资源
	KindPluginBindings = resourcestore.KindPluginBindings
	// KindAuthPolicies 表示认证策略资源
	KindAuthPolicies = resourcestore.KindAuthPolicies
	// KindRateLimitPolicies 表示限流策略资源
	KindRateLimitPolicies = resourcestore.KindRateLimitPolicies
	// KindPolicyBindings 表示策略绑定资源
	KindPolicyBindings = resourcestore.KindPolicyBindings
)

// Service 承载通用声明式资源查询用例
type Service struct {
	store *resourcestore.Store
}

// New 创建通用声明式资源 service
func New(store *resourcestore.Store) *Service {
	return &Service{store: store}
}

// List 查询资源列表
func (s *Service) List(ctx context.Context, kind Kind) (ResourceListView, error) {
	raw, err := s.store.List(ctx, kind)
	if err != nil {
		return ResourceListView{}, err
	}

	items, err := s.resourceViews(kind, raw)
	if err != nil {
		return ResourceListView{}, err
	}
	return ResourceListView{
		Kind:  kind,
		Items: items,
	}, nil
}

// Get 查询单个资源
func (s *Service) Get(ctx context.Context, kind Kind, name string) (ResourceView, error) {
	raw, err := s.store.Get(ctx, kind, name)
	if err != nil {
		return ResourceView{}, err
	}

	return s.resourceView(kind, raw)
}

func (s *Service) resourceViews(kind Kind, raw any) ([]ResourceView, error) {
	switch list := raw.(type) {
	case *gatewayv1.AIRouteList:
		return s.aiRouteViews(kind, list.Items)
	case *gatewayv1.AIProviderList:
		return s.aiProviderViews(kind, list.Items)
	case *gatewayv1.AIModelList:
		return s.aiModelViews(kind, list.Items)
	case *gatewayv1.AIPolicyList:
		return s.aiPolicyViews(kind, list.Items)
	case *gatewayv1.PluginList:
		return s.pluginViews(kind, list.Items)
	case *gatewayv1.PluginBindingList:
		return s.pluginBindingViews(kind, list.Items)
	case *gatewayv1.AuthPolicyList:
		return s.authPolicyViews(kind, list.Items)
	case *gatewayv1.RateLimitPolicyList:
		return s.rateLimitPolicyViews(kind, list.Items)
	case *gatewayv1.PolicyBindingList:
		return s.policyBindingViews(kind, list.Items)
	default:
		return nil, fmt.Errorf("unsupported admin resource list type %T", raw)
	}
}

func (s *Service) resourceView(kind Kind, raw any) (ResourceView, error) {
	switch item := raw.(type) {
	case *gatewayv1.AIRoute:
		return s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
	case *gatewayv1.AIProvider:
		return s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
	case *gatewayv1.AIModel:
		return s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
	case *gatewayv1.AIPolicy:
		return s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
	case *gatewayv1.Plugin:
		return s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
	case *gatewayv1.PluginBinding:
		return s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
	case *gatewayv1.AuthPolicy:
		return s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
	case *gatewayv1.RateLimitPolicy:
		return s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
	case *gatewayv1.PolicyBinding:
		return s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
	default:
		return ResourceView{}, fmt.Errorf("unsupported admin resource type %T", raw)
	}
}

func (s *Service) aiRouteViews(kind Kind, items []gatewayv1.AIRoute) ([]ResourceView, error) {
	views := make([]ResourceView, 0, len(items))
	for _, item := range items {
		view, err := s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) aiProviderViews(kind Kind, items []gatewayv1.AIProvider) ([]ResourceView, error) {
	views := make([]ResourceView, 0, len(items))
	for _, item := range items {
		view, err := s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) aiModelViews(kind Kind, items []gatewayv1.AIModel) ([]ResourceView, error) {
	views := make([]ResourceView, 0, len(items))
	for _, item := range items {
		view, err := s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) aiPolicyViews(kind Kind, items []gatewayv1.AIPolicy) ([]ResourceView, error) {
	views := make([]ResourceView, 0, len(items))
	for _, item := range items {
		view, err := s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) pluginViews(kind Kind, items []gatewayv1.Plugin) ([]ResourceView, error) {
	views := make([]ResourceView, 0, len(items))
	for _, item := range items {
		view, err := s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) pluginBindingViews(kind Kind, items []gatewayv1.PluginBinding) ([]ResourceView, error) {
	views := make([]ResourceView, 0, len(items))
	for _, item := range items {
		view, err := s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) authPolicyViews(kind Kind, items []gatewayv1.AuthPolicy) ([]ResourceView, error) {
	views := make([]ResourceView, 0, len(items))
	for _, item := range items {
		view, err := s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) rateLimitPolicyViews(kind Kind, items []gatewayv1.RateLimitPolicy) ([]ResourceView, error) {
	views := make([]ResourceView, 0, len(items))
	for _, item := range items {
		view, err := s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) policyBindingViews(kind Kind, items []gatewayv1.PolicyBinding) ([]ResourceView, error) {
	views := make([]ResourceView, 0, len(items))
	for _, item := range items {
		view, err := s.view(kind, &item.ObjectMeta, item.Spec, item.Status)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) view(kind Kind, metadata *metav1.ObjectMeta, spec any, status gatewayv1.ResourceStatus) (ResourceView, error) {
	specView, err := s.specView(spec)
	if err != nil {
		return ResourceView{}, err
	}
	return ResourceView{
		Kind: kind,
		Metadata: MetadataView{
			Name:            metadata.Name,
			Generation:      metadata.Generation,
			ResourceVersion: metadata.ResourceVersion,
			CreatedAt:       s.timeView(metadata.CreationTimestamp),
		},
		Spec:   specView,
		Status: s.statusView(status),
	}, nil
}

func (s *Service) specView(spec any) (map[string]any, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	view := map[string]any{}
	if err := json.Unmarshal(data, &view); err != nil {
		return nil, err
	}
	return view, nil
}

func (s *Service) statusView(status gatewayv1.ResourceStatus) StatusView {
	conditions := make([]ConditionView, 0, len(status.Conditions))
	for _, condition := range status.Conditions {
		conditions = append(conditions, ConditionView{
			Type:               condition.Type,
			Status:             string(condition.Status),
			ObservedGeneration: condition.ObservedGeneration,
			LastTransitionTime: s.timeView(condition.LastTransitionTime),
			Reason:             condition.Reason,
			Message:            condition.Message,
		})
	}
	return StatusView{Conditions: conditions}
}

func (s *Service) timeView(value metav1.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Time.Format(time.RFC3339)
}

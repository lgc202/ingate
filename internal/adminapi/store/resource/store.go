package resource

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Kind 表示 admin-api 暴露的声明式资源集合
type Kind string

const (
	// KindAIRoutes 表示 AI 路由资源
	KindAIRoutes Kind = "ai-routes"
	// KindAIProviders 表示 AI 供应商资源
	KindAIProviders Kind = "ai-providers"
	// KindAIModels 表示 AI 模型资源
	KindAIModels Kind = "ai-models"
	// KindAIPolicies 表示 AI 策略资源
	KindAIPolicies Kind = "ai-policies"
	// KindPlugins 表示插件资源
	KindPlugins Kind = "plugins"
	// KindPluginBindings 表示插件绑定资源
	KindPluginBindings Kind = "plugin-bindings"
	// KindAuthPolicies 表示认证策略资源
	KindAuthPolicies Kind = "auth-policies"
	// KindRateLimitPolicies 表示限流策略资源
	KindRateLimitPolicies Kind = "rate-limit-policies"
	// KindPolicyBindings 表示策略绑定资源
	KindPolicyBindings Kind = "policy-bindings"
)

// Store 读取 admin-api 尚未拆出专用用例的声明式资源
type Store struct {
	client clientset.Interface
}

// New 创建声明式资源 store
func New(client clientset.Interface) *Store {
	return &Store{client: client}
}

// List 查询资源列表
func (s *Store) List(ctx context.Context, kind Kind) (any, error) {
	client := s.client.GatewayV1()
	switch kind {
	case KindAIRoutes:
		return client.AIRoutes().List(ctx, metav1.ListOptions{})
	case KindAIProviders:
		return client.AIProviders().List(ctx, metav1.ListOptions{})
	case KindAIModels:
		return client.AIModels().List(ctx, metav1.ListOptions{})
	case KindAIPolicies:
		return client.AIPolicies().List(ctx, metav1.ListOptions{})
	case KindPlugins:
		return client.Plugins().List(ctx, metav1.ListOptions{})
	case KindPluginBindings:
		return client.PluginBindings().List(ctx, metav1.ListOptions{})
	case KindAuthPolicies:
		return client.AuthPolicies().List(ctx, metav1.ListOptions{})
	case KindRateLimitPolicies:
		return client.RateLimitPolicies().List(ctx, metav1.ListOptions{})
	case KindPolicyBindings:
		return client.PolicyBindings().List(ctx, metav1.ListOptions{})
	default:
		return nil, fmt.Errorf("unsupported admin resource kind %q", kind)
	}
}

// Get 查询单个资源
func (s *Store) Get(ctx context.Context, kind Kind, name string) (any, error) {
	client := s.client.GatewayV1()
	switch kind {
	case KindAIRoutes:
		return client.AIRoutes().Get(ctx, name, metav1.GetOptions{})
	case KindAIProviders:
		return client.AIProviders().Get(ctx, name, metav1.GetOptions{})
	case KindAIModels:
		return client.AIModels().Get(ctx, name, metav1.GetOptions{})
	case KindAIPolicies:
		return client.AIPolicies().Get(ctx, name, metav1.GetOptions{})
	case KindPlugins:
		return client.Plugins().Get(ctx, name, metav1.GetOptions{})
	case KindPluginBindings:
		return client.PluginBindings().Get(ctx, name, metav1.GetOptions{})
	case KindAuthPolicies:
		return client.AuthPolicies().Get(ctx, name, metav1.GetOptions{})
	case KindRateLimitPolicies:
		return client.RateLimitPolicies().Get(ctx, name, metav1.GetOptions{})
	case KindPolicyBindings:
		return client.PolicyBindings().Get(ctx, name, metav1.GetOptions{})
	default:
		return nil, fmt.Errorf("unsupported admin resource kind %q", kind)
	}
}

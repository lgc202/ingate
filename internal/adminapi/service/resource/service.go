package resource

import (
	"context"

	resourcestore "github.com/lgc202/ingate/internal/adminapi/store/resource"
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
func (s *Service) List(ctx context.Context, kind Kind) (any, error) {
	return s.store.List(ctx, kind)
}

// Get 查询单个资源
func (s *Service) Get(ctx context.Context, kind Kind, name string) (any, error) {
	return s.store.Get(ctx, kind, name)
}

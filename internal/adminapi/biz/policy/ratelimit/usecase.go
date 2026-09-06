// Package ratelimit 处理请求限流策略的业务规则和资源协作。
package ratelimit

import (
	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/resource/status"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义请求限流策略管理所需的持久化能力。
type Store interface {
	policy.Store[resource.RateLimitPolicy, resource.RateLimitPolicySpec]
}

// Usecase 提供请求限流策略管理能力。
type Usecase struct {
	*policy.Usecase[resource.RateLimitPolicy, resource.RateLimitPolicySpec]
}

// NewUsecase 创建请求限流策略用例。
func NewUsecase(
	store Store,
	gateways policy.GatewayReader,
	routes policy.RouteReader,
) *Usecase {
	return &Usecase{Usecase: policy.NewUsecase(
		store,
		policy.NewTargetResolver(gateways, routes),
		policy.ResourceAccessors[resource.RateLimitPolicy, resource.RateLimitPolicySpec]{
			DisplayName: func(item *resource.RateLimitPolicy) string { return item.Spec.DisplayName },
			Enabled:     func(item *resource.RateLimitPolicy) bool { return item.Spec.Enabled },
			Generation:  func(item *resource.RateLimitPolicy) int64 { return item.Generation },
			TargetRefs:  func(item *resource.RateLimitPolicy) []resource.PolicyTargetRef { return item.Spec.TargetRefs },
			TargetRefsFromSpec: func(spec resource.RateLimitPolicySpec) []resource.PolicyTargetRef {
				return spec.TargetRefs
			},
			Status: func(item *resource.RateLimitPolicy) status.Status {
				return policy.Status(item.Generation, item.Spec.Enabled, len(item.Spec.TargetRefs), item.Status.Conditions)
			},
		},
		nil,
	)}
}

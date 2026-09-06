// Package iprestriction 处理客户端 IP 访问限制策略的业务规则和资源协作。
package iprestriction

import (
	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/resource/status"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义客户端 IP 访问限制策略管理所需的持久化能力。
type Store interface {
	policy.Store[resource.IPRestrictionPolicy, resource.IPRestrictionPolicySpec]
}

// Usecase 提供客户端 IP 访问限制策略管理能力。
type Usecase struct {
	*policy.Usecase[resource.IPRestrictionPolicy, resource.IPRestrictionPolicySpec]
}

// NewUsecase 创建客户端 IP 访问限制策略用例。
func NewUsecase(
	store Store,
	gateways policy.GatewayReader,
	routes policy.RouteReader,
) *Usecase {
	return &Usecase{Usecase: policy.NewUsecase(
		store,
		policy.NewTargetResolver(gateways, routes),
		policy.ResourceAccessors[resource.IPRestrictionPolicy, resource.IPRestrictionPolicySpec]{
			DisplayName: func(item *resource.IPRestrictionPolicy) string { return item.Spec.DisplayName },
			Enabled:     func(item *resource.IPRestrictionPolicy) bool { return item.Spec.Enabled },
			Generation:  func(item *resource.IPRestrictionPolicy) int64 { return item.Generation },
			TargetRefs:  func(item *resource.IPRestrictionPolicy) []resource.PolicyTargetRef { return item.Spec.TargetRefs },
			TargetRefsFromSpec: func(spec resource.IPRestrictionPolicySpec) []resource.PolicyTargetRef {
				return spec.TargetRefs
			},
			Status: func(item *resource.IPRestrictionPolicy) status.Status {
				return policy.Status(item.Generation, item.Spec.Enabled, len(item.Spec.TargetRefs), item.Status.Conditions)
			},
		},
		nil,
	)}
}

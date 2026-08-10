// Package configuration 实现配置发布状态聚合用例
package configuration

import (
	"context"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// GatewayRepository 定义配置状态聚合需要的 Gateway 查询能力
type GatewayRepository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.Gateway], error)
}

// RouteRepository 定义配置状态聚合需要的 Route 查询能力
type RouteRepository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.Route], error)
}

// UpstreamRepository 定义配置状态聚合需要的 Upstream 查询能力
type UpstreamRepository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.Upstream], error)
}

// CertificateRepository 定义配置状态聚合需要的 Certificate 查询能力
type CertificateRepository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.Certificate], error)
}

// RateLimitPolicyRepository 定义配置状态聚合需要的限流策略查询能力
type RateLimitPolicyRepository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.RateLimitPolicy], error)
}

// IPRestrictionPolicyRepository 定义配置状态聚合需要的 IP 访问限制策略查询能力
type IPRestrictionPolicyRepository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.IPRestrictionPolicy], error)
}

// Usecase 承载配置状态聚合用例
type Usecase struct {
	gateways              GatewayRepository
	routes                RouteRepository
	upstreams             UpstreamRepository
	certificates          CertificateRepository
	rateLimitPolicies     RateLimitPolicyRepository
	ipRestrictionPolicies IPRestrictionPolicyRepository
}

// NewUsecase 创建配置发布状态查询用例
func NewUsecase(
	gateways GatewayRepository,
	routes RouteRepository,
	upstreams UpstreamRepository,
	certificates CertificateRepository,
	rateLimitPolicies RateLimitPolicyRepository,
	ipRestrictionPolicies IPRestrictionPolicyRepository,
) *Usecase {
	return &Usecase{
		gateways:              gateways,
		routes:                routes,
		upstreams:             upstreams,
		certificates:          certificates,
		rateLimitPolicies:     rateLimitPolicies,
		ipRestrictionPolicies: ipRestrictionPolicies,
	}
}

// Package configuration 实现配置发布状态聚合用例
package configuration

import (
	"context"
	"fmt"
	"slices"

	"golang.org/x/sync/errgroup"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// GatewayRepository 定义配置状态聚合需要的 Gateway 查询能力
type GatewayRepository interface {
	List(context.Context) ([]resource.Gateway, error)
}

// RouteRepository 定义配置状态聚合需要的 Route 查询能力
type RouteRepository interface {
	List(context.Context) ([]resource.Route, error)
}

// UpstreamRepository 定义配置状态聚合需要的 Upstream 查询能力
type UpstreamRepository interface {
	List(context.Context) ([]resource.Upstream, error)
}

// CertificateRepository 定义配置状态聚合需要的 Certificate 查询能力
type CertificateRepository interface {
	List(context.Context) ([]resource.Certificate, error)
}

// RateLimitPolicyRepository 定义配置状态聚合需要的限流策略查询能力
type RateLimitPolicyRepository interface {
	List(context.Context) ([]resource.RateLimitPolicy, error)
}

// AccessControlPolicyRepository 定义配置状态聚合需要的访问控制策略查询能力
type AccessControlPolicyRepository interface {
	List(context.Context) ([]resource.AccessControlPolicy, error)
}

// TokenQuotaPolicyRepository 定义配置状态聚合需要的 Token 配额策略查询能力
type TokenQuotaPolicyRepository interface {
	List(context.Context) ([]resource.TokenQuotaPolicy, error)
}

// Usecase 承载配置状态聚合用例
type Usecase struct {
	gateways              GatewayRepository
	routes                RouteRepository
	upstreams             UpstreamRepository
	certificates          CertificateRepository
	rateLimitPolicies     RateLimitPolicyRepository
	accessControlPolicies AccessControlPolicyRepository
	tokenQuotaPolicies    TokenQuotaPolicyRepository
}

type resourceLists struct {
	gateways              []resource.Gateway
	routes                []resource.Route
	upstreams             []resource.Upstream
	certificates          []resource.Certificate
	rateLimitPolicies     []resource.RateLimitPolicy
	accessControlPolicies []resource.AccessControlPolicy
	tokenQuotaPolicies    []resource.TokenQuotaPolicy
}

// NewUsecase 创建配置发布状态查询用例
func NewUsecase(
	gateways GatewayRepository,
	routes RouteRepository,
	upstreams UpstreamRepository,
	certificates CertificateRepository,
	rateLimitPolicies RateLimitPolicyRepository,
	accessControlPolicies AccessControlPolicyRepository,
	tokenQuotaPolicies TokenQuotaPolicyRepository,
) *Usecase {
	return &Usecase{
		gateways:              gateways,
		routes:                routes,
		upstreams:             upstreams,
		certificates:          certificates,
		rateLimitPolicies:     rateLimitPolicies,
		accessControlPolicies: accessControlPolicies,
		tokenQuotaPolicies:    tokenQuotaPolicies,
	}
}

// Get 返回全部声明式资源的产品配置状态
func (u *Usecase) Get(ctx context.Context) (*Report, error) {
	resources, err := u.listResources(ctx)
	if err != nil {
		return nil, err
	}

	total := len(resources.gateways) + len(resources.routes) + len(resources.upstreams) +
		len(resources.certificates) + len(resources.rateLimitPolicies) + len(resources.accessControlPolicies) +
		len(resources.tokenQuotaPolicies)
	items := make([]Item, 0, total)
	for _, gateway := range resources.gateways {
		items = append(items, Item{
			Kind:   resource.KindGateway,
			ID:     gateway.Name,
			Name:   displayName(gateway.Spec.DisplayName, gateway.Name),
			Status: biz.EnabledResourceStatus(gateway.Generation, gateway.Spec.Enabled, gateway.Status.Conditions),
		})
	}
	for _, route := range resources.routes {
		items = append(items, Item{
			Kind:   resource.KindRoute,
			ID:     route.Name,
			Name:   displayName(route.Spec.DisplayName, route.Name),
			Status: biz.EnabledResourceStatus(route.Generation, route.Spec.Enabled, route.Status.Conditions),
		})
	}
	for _, upstream := range resources.upstreams {
		items = append(items, Item{
			Kind:   resource.KindUpstream,
			ID:     upstream.Name,
			Name:   displayName(upstream.Spec.DisplayName, upstream.Name),
			Status: biz.ResourceStatusFromConditions(upstream.Generation, upstream.Status.Conditions),
		})
	}
	for _, certificate := range resources.certificates {
		items = append(items, Item{
			Kind:   resource.KindCertificate,
			ID:     certificate.Name,
			Name:   displayName(certificate.Spec.DisplayName, certificate.Name),
			Status: biz.ResourceStatusFromConditions(certificate.Generation, certificate.Status.Conditions),
		})
	}
	for _, policy := range resources.rateLimitPolicies {
		items = append(items, Item{
			Kind: resource.KindRateLimitPolicy,
			ID:   policy.Name,
			Name: displayName(policy.Spec.DisplayName, policy.Name),
			Status: biz.EffectivePolicyStatus(
				policy.Generation,
				policy.Spec.Enabled,
				policy.Spec.TargetRefs,
				policy.Status.Conditions,
				policy.Status.Targets,
			),
		})
	}
	for _, policy := range resources.accessControlPolicies {
		items = append(items, Item{
			Kind: resource.KindAccessControlPolicy,
			ID:   policy.Name,
			Name: displayName(policy.Spec.DisplayName, policy.Name),
			Status: biz.EffectivePolicyStatus(
				policy.Generation,
				policy.Spec.Enabled,
				policy.Spec.TargetRefs,
				policy.Status.Conditions,
				policy.Status.Targets,
			),
		})
	}
	for _, policy := range resources.tokenQuotaPolicies {
		items = append(items, Item{
			Kind: resource.KindTokenQuotaPolicy,
			ID:   policy.Name,
			Name: displayName(policy.Spec.DisplayName, policy.Name),
			Status: biz.EffectivePolicyStatus(
				policy.Generation,
				policy.Spec.Enabled,
				policy.Spec.TargetRefs,
				policy.Status.Conditions,
				policy.Status.Targets,
			),
		})
	}

	slices.SortFunc(items, compareItems)
	return &Report{Summary: summarize(items), Items: items}, nil
}

func (u *Usecase) listResources(ctx context.Context) (resourceLists, error) {
	var resources resourceLists
	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		gateways, err := u.gateways.List(ctx)
		if err != nil {
			return fmt.Errorf("list gateways: %w", err)
		}
		resources.gateways = gateways
		return nil
	})
	group.Go(func() error {
		routes, err := u.routes.List(ctx)
		if err != nil {
			return fmt.Errorf("list routes: %w", err)
		}
		resources.routes = routes
		return nil
	})
	group.Go(func() error {
		upstreams, err := u.upstreams.List(ctx)
		if err != nil {
			return fmt.Errorf("list upstreams: %w", err)
		}
		resources.upstreams = upstreams
		return nil
	})
	group.Go(func() error {
		certificates, err := u.certificates.List(ctx)
		if err != nil {
			return fmt.Errorf("list certificates: %w", err)
		}
		resources.certificates = certificates
		return nil
	})
	group.Go(func() error {
		policies, err := u.rateLimitPolicies.List(ctx)
		if err != nil {
			return fmt.Errorf("list rate limit policies: %w", err)
		}
		resources.rateLimitPolicies = policies
		return nil
	})
	group.Go(func() error {
		policies, err := u.accessControlPolicies.List(ctx)
		if err != nil {
			return fmt.Errorf("list access control policies: %w", err)
		}
		resources.accessControlPolicies = policies
		return nil
	})
	group.Go(func() error {
		policies, err := u.tokenQuotaPolicies.List(ctx)
		if err != nil {
			return fmt.Errorf("list token quota policies: %w", err)
		}
		resources.tokenQuotaPolicies = policies
		return nil
	})
	if err := group.Wait(); err != nil {
		return resourceLists{}, err
	}
	return resources, nil
}

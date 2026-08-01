// Package configurationstatus 聚合控制台需要查看的全部配置生效状态
package configurationstatus

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/lgc202/ingate/internal/admin/service/resourcestatus"
	accesscontrolpolicystore "github.com/lgc202/ingate/internal/admin/store/accesscontrolpolicy"
	certificatestore "github.com/lgc202/ingate/internal/admin/store/certificate"
	gatewaystore "github.com/lgc202/ingate/internal/admin/store/gateway"
	ratelimitpolicystore "github.com/lgc202/ingate/internal/admin/store/ratelimitpolicy"
	routestore "github.com/lgc202/ingate/internal/admin/store/route"
	tokenquotapolicystore "github.com/lgc202/ingate/internal/admin/store/tokenquotapolicy"
	upstreamstore "github.com/lgc202/ingate/internal/admin/store/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	statusPriorityError = iota
	statusPriorityPending
	statusPriorityReady
	statusPriorityDisabled
	statusPriorityUnknown
)

const (
	kindPriorityGateway = iota
	kindPriorityRoute
	kindPriorityUpstream
	kindPriorityCertificate
	kindPriorityRateLimitPolicy
	kindPriorityAccessControlPolicy
	kindPriorityTokenQuotaPolicy
	kindPriorityUnknown
)

// Service 承载配置状态聚合用例
type Service struct {
	gateways              *gatewaystore.Store
	routes                *routestore.Store
	upstreams             *upstreamstore.Store
	certificates          *certificatestore.Store
	rateLimitPolicies     *ratelimitpolicystore.Store
	accessControlPolicies *accesscontrolpolicystore.Store
	tokenQuotaPolicies    *tokenquotapolicystore.Store
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

// New 创建配置状态聚合 service
func New(
	gateways *gatewaystore.Store,
	routes *routestore.Store,
	upstreams *upstreamstore.Store,
	certificates *certificatestore.Store,
	rateLimitPolicies *ratelimitpolicystore.Store,
	accessControlPolicies *accesscontrolpolicystore.Store,
	tokenQuotaPolicies *tokenquotapolicystore.Store,
) *Service {
	return &Service{
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
func (s *Service) Get(ctx context.Context) (*Report, error) {
	resources, err := s.listResources(ctx)
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
			Status: enabledResourceStatus(gateway.Generation, gateway.Spec.Enabled, gateway.Status.Conditions),
		})
	}
	for _, route := range resources.routes {
		items = append(items, Item{
			Kind:   resource.KindRoute,
			ID:     route.Name,
			Name:   displayName(route.Spec.DisplayName, route.Name),
			Status: enabledResourceStatus(route.Generation, route.Spec.Enabled, route.Status.Conditions),
		})
	}
	for _, upstream := range resources.upstreams {
		items = append(items, Item{
			Kind:   resource.KindUpstream,
			ID:     upstream.Name,
			Name:   displayName(upstream.Spec.DisplayName, upstream.Name),
			Status: resourcestatus.FromConditions(upstream.Generation, upstream.Status.Conditions),
		})
	}
	for _, certificate := range resources.certificates {
		items = append(items, Item{
			Kind:   resource.KindCertificate,
			ID:     certificate.Name,
			Name:   displayName(certificate.Spec.DisplayName, certificate.Name),
			Status: resourcestatus.FromConditions(certificate.Generation, certificate.Status.Conditions),
		})
	}
	for _, policy := range resources.rateLimitPolicies {
		items = append(items, Item{
			Kind: resource.KindRateLimitPolicy,
			ID:   policy.Name,
			Name: displayName(policy.Spec.DisplayName, policy.Name),
			Status: effectivePolicyStatus(
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
			Status: effectivePolicyStatus(
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
			Status: effectivePolicyStatus(
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

func (s *Service) listResources(ctx context.Context) (resourceLists, error) {
	var resources resourceLists
	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		gateways, err := s.gateways.List(ctx)
		if err != nil {
			return fmt.Errorf("list gateways: %w", err)
		}
		resources.gateways = gateways.Items
		return nil
	})
	group.Go(func() error {
		routes, err := s.routes.List(ctx)
		if err != nil {
			return fmt.Errorf("list routes: %w", err)
		}
		resources.routes = routes.Items
		return nil
	})
	group.Go(func() error {
		upstreams, err := s.upstreams.List(ctx)
		if err != nil {
			return fmt.Errorf("list upstreams: %w", err)
		}
		resources.upstreams = upstreams.Items
		return nil
	})
	group.Go(func() error {
		certificates, err := s.certificates.List(ctx)
		if err != nil {
			return fmt.Errorf("list certificates: %w", err)
		}
		resources.certificates = certificates.Items
		return nil
	})
	group.Go(func() error {
		policies, err := s.rateLimitPolicies.List(ctx)
		if err != nil {
			return fmt.Errorf("list rate limit policies: %w", err)
		}
		resources.rateLimitPolicies = policies.Items
		return nil
	})
	group.Go(func() error {
		policies, err := s.accessControlPolicies.List(ctx)
		if err != nil {
			return fmt.Errorf("list access control policies: %w", err)
		}
		resources.accessControlPolicies = policies.Items
		return nil
	})
	group.Go(func() error {
		policies, err := s.tokenQuotaPolicies.List(ctx)
		if err != nil {
			return fmt.Errorf("list token quota policies: %w", err)
		}
		resources.tokenQuotaPolicies = policies.Items
		return nil
	})
	if err := group.Wait(); err != nil {
		return resourceLists{}, err
	}
	return resources, nil
}

func enabledResourceStatus(generation int64, enabled bool, conditions []metav1.Condition) resourcestatus.Status {
	if !enabled && resourcestatus.ConfigurationApplied(generation, conditions) {
		return resourcestatus.Disabled()
	}
	return resourcestatus.FromConditions(generation, conditions)
}

func effectivePolicyStatus(
	generation int64,
	enabled bool,
	refs []resource.PolicyTargetRef,
	conditions []metav1.Condition,
	targets []resource.PolicyTargetStatus,
) resourcestatus.Status {
	if !enabled && resourcestatus.ConfigurationApplied(generation, conditions) {
		return resourcestatus.Disabled()
	}

	status := resourcestatus.ForPolicy(generation, len(refs), conditions)
	for _, ref := range refs {
		targetStatus := resourcestatus.ForPolicyTarget(generation, targetConditions(targets, ref))
		if statusPriority(targetStatus.State) < statusPriority(status.State) {
			status = targetStatus
		}
	}
	return status
}

func targetConditions(targets []resource.PolicyTargetStatus, ref resource.PolicyTargetRef) []metav1.Condition {
	for _, target := range targets {
		if target.TargetRef.Kind == ref.Kind && target.TargetRef.Name == ref.Name {
			return target.Conditions
		}
	}
	return nil
}

func summarize(items []Item) Summary {
	summary := Summary{Total: len(items)}
	for _, item := range items {
		switch item.Status.State {
		case resourcestatus.StateReady:
			summary.Ready++
		case resourcestatus.StatePending:
			summary.Pending++
		case resourcestatus.StateError:
			summary.Error++
		case resourcestatus.StateDisabled:
			summary.Disabled++
		}
	}
	return summary
}

func compareItems(a, b Item) int {
	if result := cmp.Compare(statusPriority(a.Status.State), statusPriority(b.Status.State)); result != 0 {
		return result
	}
	if result := cmp.Compare(kindPriority(a.Kind), kindPriority(b.Kind)); result != 0 {
		return result
	}
	if result := cmp.Compare(a.Name, b.Name); result != 0 {
		return result
	}
	return cmp.Compare(a.ID, b.ID)
}

func kindPriority(kind resource.Kind) int {
	switch kind {
	case resource.KindGateway:
		return kindPriorityGateway
	case resource.KindRoute:
		return kindPriorityRoute
	case resource.KindUpstream:
		return kindPriorityUpstream
	case resource.KindCertificate:
		return kindPriorityCertificate
	case resource.KindRateLimitPolicy:
		return kindPriorityRateLimitPolicy
	case resource.KindAccessControlPolicy:
		return kindPriorityAccessControlPolicy
	case resource.KindTokenQuotaPolicy:
		return kindPriorityTokenQuotaPolicy
	default:
		return kindPriorityUnknown
	}
}

func statusPriority(state resourcestatus.State) int {
	switch state {
	case resourcestatus.StateError:
		return statusPriorityError
	case resourcestatus.StatePending:
		return statusPriorityPending
	case resourcestatus.StateReady:
		return statusPriorityReady
	case resourcestatus.StateDisabled:
		return statusPriorityDisabled
	default:
		return statusPriorityUnknown
	}
}

func displayName(name, id string) string {
	if name != "" {
		return name
	}
	return id
}

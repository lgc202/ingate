package configuration

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func gatewayItem(gateway resource.Gateway) Item {
	return Item{
		Kind:   resource.KindGateway,
		ID:     gateway.Name,
		Name:   displayName(gateway.Spec.DisplayName, gateway.Name),
		Status: biz.EnabledResourceStatus(gateway.Generation, gateway.Spec.Enabled, gateway.Status.Conditions),
	}
}

func routeItem(route resource.Route) Item {
	return Item{
		Kind:   resource.KindRoute,
		ID:     route.Name,
		Name:   displayName(route.Spec.DisplayName, route.Name),
		Status: biz.EnabledResourceStatus(route.Generation, route.Spec.Enabled, route.Status.Conditions),
	}
}

func upstreamItem(upstream resource.Upstream) Item {
	return Item{
		Kind:   resource.KindUpstream,
		ID:     upstream.Name,
		Name:   displayName(upstream.Spec.DisplayName, upstream.Name),
		Status: biz.ResourceStatusFromConditions(upstream.Generation, upstream.Status.Conditions),
	}
}

func certificateItem(certificate resource.Certificate) Item {
	return Item{
		Kind:   resource.KindCertificate,
		ID:     certificate.Name,
		Name:   displayName(certificate.Spec.DisplayName, certificate.Name),
		Status: biz.ResourceStatusFromConditions(certificate.Generation, certificate.Status.Conditions),
	}
}

func rateLimitPolicyItem(policy resource.RateLimitPolicy) Item {
	return policyItem(
		resource.KindRateLimitPolicy,
		policy.Name,
		policy.Spec.DisplayName,
		policy.Generation,
		policy.Spec.Enabled,
		policy.Spec.TargetRefs,
		policy.Status.Conditions,
		policy.Status.Targets,
	)
}

func ipRestrictionPolicyItem(policy resource.IPRestrictionPolicy) Item {
	return policyItem(
		resource.KindIPRestrictionPolicy,
		policy.Name,
		policy.Spec.DisplayName,
		policy.Generation,
		policy.Spec.Enabled,
		policy.Spec.TargetRefs,
		policy.Status.Conditions,
		policy.Status.Targets,
	)
}

func policyItem(
	kind resource.Kind,
	id string,
	name string,
	generation int64,
	enabled bool,
	targetRefs []resource.PolicyTargetRef,
	conditions []metav1.Condition,
	targets []resource.PolicyTargetStatus,
) Item {
	return Item{
		Kind: kind,
		ID:   id,
		Name: displayName(name, id),
		Status: biz.EffectivePolicyStatus(
			generation,
			enabled,
			targetRefs,
			conditions,
			targets,
		),
	}
}

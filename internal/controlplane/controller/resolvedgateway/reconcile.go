package resolvedgateway

import (
	"context"
	"fmt"

	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
)

func (c *Controller) Reconcile(ctx context.Context, key shared.ObjectKey) error {
	if c == nil || c.loader == nil || c.persister == nil || c.status == nil {
		return fmt.Errorf("resolvedgateway controller is not fully initialized")
	}

	bundle, err := c.loader.Load(key)
	if err != nil {
		_ = c.status.MarkFailure(ctx, key, err)
		return err
	}

	resolvedGateway, err := Build(bundle)
	if err != nil {
		_ = c.status.MarkFailure(ctx, key, err)
		return err
	}

	persisted, err := c.persister.Upsert(ctx, resolvedGateway)
	if err != nil {
		_ = c.status.MarkFailure(ctx, key, err)
		return err
	}

	if err := c.status.MarkSuccess(
		ctx,
		bundle.Gateway,
		bundle.Routes,
		bundle.Backends,
		bundle.Certificates,
		collectAuthPolicies(bundle),
		collectTrafficPolicies(bundle),
		persisted,
	); err != nil {
		return err
	}
	return nil
}

func collectAuthPolicies(bundle *ResourceBundle) []*policyv1alpha1.AuthPolicy {
	seen := map[string]*policyv1alpha1.AuthPolicy{}
	for _, policy := range bundle.GatewayAuthPolicies {
		if policy != nil && policy.Name != "" {
			seen[policy.Name] = policy
		}
	}
	for _, items := range bundle.RouteAuthPolicies {
		for _, policy := range items {
			if policy != nil && policy.Name != "" {
				seen[policy.Name] = policy
			}
		}
	}
	for _, items := range bundle.BackendAuthPolicies {
		for _, policy := range items {
			if policy != nil && policy.Name != "" {
				seen[policy.Name] = policy
			}
		}
	}
	result := make([]*policyv1alpha1.AuthPolicy, 0, len(seen))
	for _, policy := range seen {
		result = append(result, policy)
	}
	return result
}

func collectTrafficPolicies(bundle *ResourceBundle) []*policyv1alpha1.TrafficPolicy {
	seen := map[string]*policyv1alpha1.TrafficPolicy{}
	for _, policy := range bundle.GatewayTrafficPolicies {
		if policy != nil && policy.Name != "" {
			seen[policy.Name] = policy
		}
	}
	for _, items := range bundle.RouteTrafficPolicies {
		for _, policy := range items {
			if policy != nil && policy.Name != "" {
				seen[policy.Name] = policy
			}
		}
	}
	for _, items := range bundle.BackendTrafficPolicies {
		for _, policy := range items {
			if policy != nil && policy.Name != "" {
				seen[policy.Name] = policy
			}
		}
	}
	result := make([]*policyv1alpha1.TrafficPolicy, 0, len(seen))
	for _, policy := range seen {
		result = append(result, policy)
	}
	return result
}

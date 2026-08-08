package service

import (
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func policyTargetRefs(targets []*adminv1.PolicyTargetRef) ([]resource.PolicyTargetRef, error) {
	refs := make([]resource.PolicyTargetRef, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if target == nil {
			return nil, badRequest("策略作用目标不能为空")
		}
		kind := resource.Kind(target.GetKind())
		id := strings.TrimSpace(target.GetId())
		if kind != resource.KindGateway && kind != resource.KindRoute {
			return nil, badRequest("策略作用目标只支持网关或路由")
		}
		key := string(kind) + "\x00" + id
		if _, exists := seen[key]; exists {
			return nil, badRequest("策略作用目标不能重复")
		}
		seen[key] = struct{}{}
		refs = append(refs, resource.PolicyTargetRef{Kind: kind, Name: id})
	}
	return refs, nil
}

func policyStatus(
	generation int64,
	enabled bool,
	targetCount int,
	conditions []metav1.Condition,
) biz.ResourceStatus {
	if !enabled && biz.ConfigurationApplied(generation, conditions) {
		return biz.DisabledResourceStatus()
	}
	return biz.PolicyResourceStatus(generation, targetCount, conditions)
}

func policyTargets(
	generation int64,
	disabled bool,
	refs []resource.PolicyTargetRef,
	statuses []resource.PolicyTargetStatus,
	names biz.PolicyTargetNames,
) []*adminv1.PolicyTarget {
	targets := make([]*adminv1.PolicyTarget, 0, len(refs))
	for _, ref := range refs {
		status := biz.PolicyTargetResourceStatus(generation, targetConditions(statuses, ref))
		if disabled {
			status = biz.DisabledResourceStatus()
		}
		targets = append(targets, &adminv1.PolicyTarget{
			Kind:        string(ref.Kind),
			Id:          ref.Name,
			DisplayName: names.Name(ref),
			Status:      resourceStatus(status),
		})
	}
	return targets
}

func targetConditions(statuses []resource.PolicyTargetStatus, ref resource.PolicyTargetRef) []metav1.Condition {
	for _, status := range statuses {
		if status.TargetRef.Kind == ref.Kind && status.TargetRef.Name == ref.Name {
			return status.Conditions
		}
	}
	return nil
}

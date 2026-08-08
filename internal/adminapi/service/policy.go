package service

import (
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
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

func policyTargets(
	generation int64,
	disabled bool,
	refs []resource.PolicyTargetRef,
	statuses []resource.PolicyTargetStatus,
	names biz.PolicyTargetNames,
) []*adminv1.PolicyTarget {
	targets := make([]*adminv1.PolicyTarget, 0, len(refs))
	for _, ref := range refs {
		status := biz.PolicyTargetStatus(generation, disabled, ref, statuses)
		targets = append(targets, &adminv1.PolicyTarget{
			Kind:        string(ref.Kind),
			Id:          ref.Name,
			DisplayName: names.Name(ref),
			Status:      resourceStatus(status),
		})
	}
	return targets
}

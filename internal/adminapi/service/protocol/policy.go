package protocol

import (
	"slices"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/policyconfig"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

// PolicyTargetRefs 校验并转换具体策略允许的作用目标。
func PolicyTargetRefs(
	targets []*adminv1.PolicyTargetRef,
	allowedKinds ...resource.Kind,
) ([]resource.PolicyTargetRef, error) {
	if len(targets) > policyconfig.MaxTargets {
		return nil, adminv1.ErrorInvalidArgument("策略作用目标数量超过限制")
	}

	refs := make([]resource.PolicyTargetRef, len(targets))
	seen := make(map[resource.PolicyTargetRef]bool, len(targets))
	for i, target := range targets {
		if target == nil {
			return nil, adminv1.ErrorInvalidArgument("策略作用目标不能为空")
		}
		kind, err := parsePolicyTargetKind(target.GetKind())
		if err != nil {
			return nil, err
		}
		if !allowedPolicyTargetKind(kind, allowedKinds) {
			return nil, adminv1.ErrorInvalidArgument("策略作用目标类型不正确")
		}
		targetID, valid := resourceconfig.NormalizeID(target.GetId())
		if !valid {
			return nil, adminv1.ErrorInvalidArgument("策略作用目标 ID 不正确")
		}

		ref := resource.PolicyTargetRef{Kind: kind, Name: targetID}
		if seen[ref] {
			return nil, adminv1.ErrorInvalidArgument("策略作用目标不能重复")
		}
		seen[ref] = true
		refs[i] = ref
	}
	return refs, nil
}

// PolicyTargetResponses 把策略目标及其生效状态转换为控制台协议。
func PolicyTargetResponses(
	generation int64,
	disabled bool,
	refs []resource.PolicyTargetRef,
	statuses []resource.PolicyTargetStatus,
	names policy.TargetNames,
) []*adminv1.PolicyTarget {
	targets := make([]*adminv1.PolicyTarget, len(refs))
	for i, ref := range refs {
		status := policy.TargetStatus(generation, disabled, ref, statuses)
		targets[i] = &adminv1.PolicyTarget{
			Kind:    policyTargetKindResponse(ref.Kind),
			Id:      ref.Name,
			Name:    names.Name(ref),
			State:   ResourceState(status.State),
			Message: ResourceMessage(status.Reason),
		}
	}
	return targets
}

func parsePolicyTargetKind(kind adminv1.PolicyTargetKind) (resource.Kind, error) {
	switch kind {
	case adminv1.PolicyTargetKind_POLICY_TARGET_KIND_GATEWAY:
		return resource.KindGateway, nil
	case adminv1.PolicyTargetKind_POLICY_TARGET_KIND_ROUTE:
		return resource.KindRoute, nil
	case adminv1.PolicyTargetKind_POLICY_TARGET_KIND_CALLER:
		return resource.KindCaller, nil
	default:
		return "", adminv1.ErrorInvalidArgument("策略作用目标类型不正确")
	}
}

func allowedPolicyTargetKind(kind resource.Kind, allowed []resource.Kind) bool {
	return slices.Contains(allowed, kind)
}

func policyTargetKindResponse(kind resource.Kind) adminv1.PolicyTargetKind {
	switch kind {
	case resource.KindGateway:
		return adminv1.PolicyTargetKind_POLICY_TARGET_KIND_GATEWAY
	case resource.KindRoute:
		return adminv1.PolicyTargetKind_POLICY_TARGET_KIND_ROUTE
	case resource.KindCaller:
		return adminv1.PolicyTargetKind_POLICY_TARGET_KIND_CALLER
	default:
		return adminv1.PolicyTargetKind_POLICY_TARGET_KIND_UNSPECIFIED
	}
}

package service

import (
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// PolicyTargetRefs 校验并转换具体策略允许的作用目标
func PolicyTargetRefs(
	targets []*adminv1.PolicyTargetRef,
	allowedKinds ...resource.Kind,
) ([]resource.PolicyTargetRef, error) {
	refs := make([]resource.PolicyTargetRef, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if target == nil {
			return nil, BadRequest("策略作用目标不能为空")
		}
		kind, err := policyTargetKind(target.GetKind())
		if err != nil {
			return nil, err
		}
		if !allowedPolicyTargetKind(kind, allowedKinds) {
			return nil, BadRequest("策略作用目标类型不正确")
		}
		id := strings.TrimSpace(target.GetId())
		key := string(kind) + "\x00" + id
		if _, exists := seen[key]; exists {
			return nil, BadRequest("策略作用目标不能重复")
		}
		seen[key] = struct{}{}
		refs = append(refs, resource.PolicyTargetRef{Kind: kind, Name: id})
	}
	return refs, nil
}

func allowedPolicyTargetKind(kind resource.Kind, allowed []resource.Kind) bool {
	for _, candidate := range allowed {
		if kind == candidate {
			return true
		}
	}
	return false
}

// PolicyTargetResponses 把策略目标及其生效状态转换为控制台协议
func PolicyTargetResponses(
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
			Kind:    policyTargetKindResponse(ref.Kind),
			Id:      ref.Name,
			Name:    names.Name(ref),
			State:   ResourceState(status.State),
			Message: ResourceMessage(status.Reason),
		})
	}
	return targets
}

func policyTargetKind(kind adminv1.PolicyTargetKind) (resource.Kind, error) {
	switch kind {
	case adminv1.PolicyTargetKind_POLICY_TARGET_KIND_GATEWAY:
		return resource.KindGateway, nil
	case adminv1.PolicyTargetKind_POLICY_TARGET_KIND_ROUTE:
		return resource.KindRoute, nil
	case adminv1.PolicyTargetKind_POLICY_TARGET_KIND_CALLER:
		return resource.KindCaller, nil
	default:
		return "", BadRequest("策略作用目标类型不正确")
	}
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

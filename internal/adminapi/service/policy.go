package service

import (
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// BuildPolicyTargetRefs 校验并转换策略作用目标
func BuildPolicyTargetRefs(targets []*adminv1.PolicyTargetRef) ([]resource.PolicyTargetRef, error) {
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

// NewPolicyTargets 把策略目标及其生效状态转换为控制台协议
func NewPolicyTargets(
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
			Kind:    newPolicyTargetKind(ref.Kind),
			Id:      ref.Name,
			Name:    names.Name(ref),
			State:   NewResourceState(status.State),
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
	default:
		return "", BadRequest("策略作用目标只支持网关或路由")
	}
}

func newPolicyTargetKind(kind resource.Kind) adminv1.PolicyTargetKind {
	switch kind {
	case resource.KindGateway:
		return adminv1.PolicyTargetKind_POLICY_TARGET_KIND_GATEWAY
	case resource.KindRoute:
		return adminv1.PolicyTargetKind_POLICY_TARGET_KIND_ROUTE
	default:
		return adminv1.PolicyTargetKind_POLICY_TARGET_KIND_UNSPECIFIED
	}
}

// Package target 转换并校验策略作用目标。
package target

import (
	"slices"

	"github.com/samber/lo"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	policybiz "github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/service/conversion"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	apivalidation "github.com/lgc202/ingate/internal/pkg/apis/gateway/validation"
)

// Refs 校验并转换具体策略允许的作用目标。
func Refs(
	targets []*adminv1.PolicyTargetRef,
	allowedKinds ...resource.Kind,
) ([]resource.PolicyTargetRef, error) {
	if len(targets) > apivalidation.MaxTargets {
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
		targetID, valid := apivalidation.NormalizeID(target.GetId())
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

// Responses 把策略目标及其生效状态转换为控制台协议。
func Responses(
	generation int64,
	disabled bool,
	refs []resource.PolicyTargetRef,
	statuses []resource.PolicyTargetStatus,
	names policybiz.TargetNames,
) []*adminv1.PolicyTarget {
	return lo.Map(refs, func(ref resource.PolicyTargetRef, _ int) *adminv1.PolicyTarget {
		status := policybiz.TargetStatus(generation, disabled, ref, statuses)
		return &adminv1.PolicyTarget{
			Kind:    policyTargetKindResponse(ref.Kind),
			Id:      ref.Name,
			Name:    names.Name(ref),
			State:   conversion.ResourceState(status.State),
			Message: conversion.ResourceMessage(status.Reason),
		}
	})
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

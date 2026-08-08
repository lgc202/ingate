// Package service 实现 Admin API 的传输协议适配
package service

import (
	"errors"
	"strings"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/wire"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const userMessageMetadata = "user_message"

// ProviderSet 汇总 Admin API 的协议服务
var ProviderSet = wire.NewSet(
	NewGatewayService,
	NewRouteService,
	NewUpstreamService,
	NewCertificateService,
	NewAccessKeyService,
	NewRateLimitPolicyService,
	NewAccessControlPolicyService,
	NewTokenQuotaPolicyService,
	NewConfigurationService,
	NewHealthService,
)

func badRequest(message string) error {
	return kratoserrors.BadRequest(adminv1.ErrorReason_INVALID_ARGUMENT.String(), "invalid request").WithMetadata(map[string]string{
		userMessageMetadata: message,
	})
}

func operationError(err error, message string) error {
	if err == nil {
		return nil
	}
	var serviceError *kratoserrors.Error
	if errors.As(err, &serviceError) {
		return err
	}
	return kratoserrors.InternalServer(adminv1.ErrorReason_INTERNAL_ERROR.String(), "operation failed").
		WithMetadata(map[string]string{userMessageMetadata: message}).
		WithCause(err)
}

func validateID(id string) error {
	if strings.TrimSpace(id) == "" {
		return badRequest("资源 ID 不能为空")
	}
	return nil
}

func timestamp(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

func optionalTime(value *timestamppb.Timestamp) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	if err := value.CheckValid(); err != nil {
		return nil, badRequest("时间格式不正确")
	}
	result := value.AsTime()
	return &result, nil
}

func resourceStatus(status biz.ResourceStatus) *adminv1.ResourceStatus {
	message := ""
	switch status.Reason {
	case biz.ReasonAwaitingAcceptance:
		message = "配置正在处理中"
	case biz.ReasonCheckingReferences:
		message = "正在检查关联资源"
	case biz.ReasonProgramming:
		message = "配置正在生效"
	case biz.ReasonReady:
		message = "配置已生效"
	case biz.ReasonDisabled:
		message = "已停用"
	case biz.ReasonUnapplied:
		message = "策略已保存，尚未应用"
	case biz.ReasonTargetNotApplied:
		message = "目标当前没有可生效的流量入口"
	case biz.ReasonInvalidSpec:
		message = "配置内容不正确"
	case biz.ReasonReferenceNotFound:
		message = "引用的资源不存在"
	case biz.ReasonInvalidReference:
		message = "引用的资源不可用"
	case biz.ReasonConflict:
		message = "配置与其他资源冲突"
	case biz.ReasonUnsupported:
		message = "当前版本尚不支持该配置"
	case biz.ReasonCompileFailed:
		message = "配置处理失败"
	case biz.ReasonRejected:
		message = "配置未能生效"
	case biz.ReasonDeliveryFailed:
		message = "配置发布失败"
	}
	return &adminv1.ResourceStatus{State: string(status.State), Message: message}
}

func policyTargetRefs(targets []*adminv1.PolicyTargetRef) ([]resource.PolicyTargetRef, error) {
	refs := make([]resource.PolicyTargetRef, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if target == nil {
			return nil, badRequest("策略作用目标不能为空")
		}
		kind := resource.Kind(target.GetKind())
		id := strings.TrimSpace(target.GetId())
		if id == "" {
			return nil, badRequest("策略作用目标不能为空")
		}
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

package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ConditionType 表示声明式资源的稳定状态类型
type ConditionType string

const (
	// ConditionAccepted 表示资源当前版本是否被控制面接受
	ConditionAccepted ConditionType = "Accepted"
	// ConditionResolvedRefs 表示资源当前版本的引用是否均已解析
	ConditionResolvedRefs ConditionType = "ResolvedRefs"
	// ConditionProgrammed 表示资源当前版本是否已进入生效配置
	ConditionProgrammed ConditionType = "Programmed"
)

// ConditionReason 表示声明式资源状态变化的稳定原因
type ConditionReason string

const (
	// ReasonAccepted 表示资源已被控制面接受
	ReasonAccepted ConditionReason = "Accepted"
	// ReasonResolvedRefs 表示资源引用均已解析
	ReasonResolvedRefs ConditionReason = "ResolvedRefs"
	// ReasonProgrammed 表示资源配置已经生效
	ReasonProgrammed ConditionReason = "Programmed"
	// ReasonInvalidSpec 表示资源字段不满足要求
	ReasonInvalidSpec ConditionReason = "InvalidSpec"
	// ReasonReferenceNotFound 表示资源引用的目标不存在
	ReasonReferenceNotFound ConditionReason = "ReferenceNotFound"
	// ReasonInvalidReference 表示资源引用的目标存在但不可用
	ReasonInvalidReference ConditionReason = "InvalidReference"
	// ReasonConflict 表示资源与配置域中的其他声明冲突
	ReasonConflict ConditionReason = "Conflict"
	// ReasonUnsupported 表示当前控制面不支持该配置
	ReasonUnsupported ConditionReason = "Unsupported"
	// ReasonCompileFailed 表示资源未能生成一致的网关配置
	ReasonCompileFailed ConditionReason = "CompileFailed"
	// ReasonPending 表示资源配置仍在等待生效
	ReasonPending ConditionReason = "Pending"
	// ReasonNotApplied 表示策略当前没有可生效的目标
	ReasonNotApplied ConditionReason = "NotApplied"
	// ReasonRejected 表示资源配置被网关实例拒绝
	ReasonRejected ConditionReason = "Rejected"
	// ReasonDeliveryFailed 表示资源配置发布失败
	ReasonDeliveryFailed ConditionReason = "DeliveryFailed"
)

// Kind 表示声明式资源类型
type Kind string

const (
	// KindGateway 表示 Gateway 资源类型
	KindGateway Kind = "Gateway"
	// KindRoute 表示 Route 资源类型
	KindRoute Kind = "Route"
	// KindUpstream 表示 Upstream 资源类型
	KindUpstream Kind = "Upstream"
	// KindCertificate 表示 Certificate 资源类型
	KindCertificate Kind = "Certificate"
	// KindRateLimitPolicy 表示 RateLimitPolicy 资源类型
	KindRateLimitPolicy Kind = "RateLimitPolicy"
	// KindIPRestrictionPolicy 表示 IPRestrictionPolicy 资源类型
	KindIPRestrictionPolicy Kind = "IPRestrictionPolicy"
	// KindCaller 表示 Caller 资源类型
	KindCaller Kind = "Caller"
	// KindTokenQuotaPolicy 表示 TokenQuotaPolicy 资源类型
	KindTokenQuotaPolicy Kind = "TokenQuotaPolicy"
)

// PolicyTargetRef 表示策略的生效目标
type PolicyTargetRef struct {
	// Kind 由具体策略约束可引用的资源类型
	Kind Kind `json:"kind"`
	// Name 引用目标资源的 metadata.name
	Name string `json:"name"`
}

// PolicyStatus 表示策略的总体状态和各目标生效状态
type PolicyStatus struct {
	// Conditions 记录策略整体的接受、引用解析和生效结果
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Targets 保留每个目标的独立结果，单个目标失败不影响其他目标
	// +listType=atomic
	Targets []PolicyTargetStatus `json:"targets,omitempty"`
}

// PolicyTargetStatus 表示策略在单个目标上的生效状态
type PolicyTargetStatus struct {
	// TargetRef 标识当前状态对应的策略目标
	TargetRef PolicyTargetRef `json:"targetRef"`
	// Conditions 记录策略在当前目标上的解析和生效结果
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ResourceStatus 表示声明式资源状态
type ResourceStatus struct {
	// Conditions 记录资源当前 Generation 的接受、引用解析和生效结果
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

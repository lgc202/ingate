package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	// KindWasmPlugin 表示 WasmPlugin 资源类型
	KindWasmPlugin Kind = "WasmPlugin"
	// KindPluginSource 表示 PluginSource 资源类型
	KindPluginSource Kind = "PluginSource"
	// KindHeaderTransformationPolicy 表示 HeaderTransformationPolicy 资源类型
	KindHeaderTransformationPolicy Kind = "HeaderTransformationPolicy"
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

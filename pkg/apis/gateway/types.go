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
	// KindTokenQuotaPolicy 表示 TokenQuotaPolicy 资源类型
	KindTokenQuotaPolicy Kind = "TokenQuotaPolicy"
)

// PolicyTargetRef 表示策略的生效目标
type PolicyTargetRef struct {
	Kind Kind   `json:"kind"`
	Name string `json:"name"`
}

// PolicyStatus 表示策略的总体状态和各目标生效状态
type PolicyStatus struct {
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +listType=atomic
	Targets []PolicyTargetStatus `json:"targets,omitempty"`
}

// PolicyTargetStatus 表示策略在单个目标上的生效状态
type PolicyTargetStatus struct {
	TargetRef PolicyTargetRef `json:"targetRef"`
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ResourceStatus 表示声明式资源状态
type ResourceStatus struct {
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

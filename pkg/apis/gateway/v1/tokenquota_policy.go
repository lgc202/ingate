package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	// TokenQuotaMaxTokens 是 Redis Lua 与前端都能精确表达的最大 Token 配额
	TokenQuotaMaxTokens int64 = 1<<53 - 1
	// TokenQuotaMaxWindowSeconds 是数据面窗口秒数的最大可执行值
	TokenQuotaMaxWindowSeconds int64 = 1<<31 - 1
)

// TokenQuotaSubjectType 表示 Token 配额的共享维度
type TokenQuotaSubjectType string

const (
	// TokenQuotaSubjectTypeShared 表示策略目标内的请求共享同一个预算池
	TokenQuotaSubjectTypeShared TokenQuotaSubjectType = "Shared"
	// TokenQuotaSubjectTypeIP 表示按网关看到的来源 IP 区分预算池
	TokenQuotaSubjectTypeIP TokenQuotaSubjectType = "IP"
	// TokenQuotaSubjectTypeHeader 表示按请求 Header 区分预算池
	TokenQuotaSubjectTypeHeader TokenQuotaSubjectType = "Header"
)

// TokenQuotaFailurePolicy 表示 Token 配额执行异常时的处理方式
type TokenQuotaFailurePolicy string

const (
	// TokenQuotaFailurePolicyFailOpen 表示配额执行失败时放行请求
	TokenQuotaFailurePolicyFailOpen TokenQuotaFailurePolicy = "FailOpen"
	// TokenQuotaFailurePolicyFailClose 表示配额执行失败时拒绝请求
	TokenQuotaFailurePolicyFailClose TokenQuotaFailurePolicy = "FailClose"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced

// TokenQuotaPolicy 声明 AI 模型流量的 Token 配额策略
type TokenQuotaPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TokenQuotaPolicySpec `json:"spec,omitempty"`
	Status PolicyStatus         `json:"status,omitempty"`
}

// TokenQuotaPolicyList 表示 TokenQuotaPolicy 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TokenQuotaPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []TokenQuotaPolicy `json:"items"`
}

// TokenQuotaPolicySpec 定义单个共享预算池及其生效目标
type TokenQuotaPolicySpec struct {
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
	// +listType=atomic
	TargetRefs    []PolicyTargetRef       `json:"targetRefs,omitempty"`
	Subject       TokenQuotaSubject       `json:"subject"`
	Quota         TokenQuota              `json:"quota"`
	FailurePolicy TokenQuotaFailurePolicy `json:"failurePolicy"`
	Response      TokenQuotaResponse      `json:"response,omitempty"`
}

// TokenQuotaSubject 定义请求如何映射到预算池
type TokenQuotaSubject struct {
	Type       TokenQuotaSubjectType `json:"type"`
	HeaderName string                `json:"headerName,omitempty"`
}

// TokenQuota 定义一个时间窗口内允许消费的 Token 数量
type TokenQuota struct {
	Tokens        int64 `json:"tokens"`
	WindowSeconds int64 `json:"windowSeconds"`
}

// TokenQuotaResponse 定义超过 Token 配额时返回给调用方的响应
type TokenQuotaResponse struct {
	Message string `json:"message,omitempty"`
}

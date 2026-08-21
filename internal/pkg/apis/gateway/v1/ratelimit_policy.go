package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	// RateLimitMaxRequests 是单条限流额度允许的最大请求数
	RateLimitMaxRequests int64 = 1<<31 - 1
	// RateLimitMaxWindowSeconds 是单条限流窗口允许的最大秒数
	RateLimitMaxWindowSeconds int64 = 1<<31 - 1
)

// RateLimitSubjectType 表示额度在目标内如何划分计数对象
type RateLimitSubjectType string

const (
	// RateLimitSubjectShared 表示目标内所有请求共享额度
	RateLimitSubjectShared RateLimitSubjectType = "Shared"
	// RateLimitSubjectIP 表示每个客户端 IP 独立使用额度
	RateLimitSubjectIP RateLimitSubjectType = "IP"
	// RateLimitSubjectHeader 表示每个请求 Header 值独立使用额度
	RateLimitSubjectHeader RateLimitSubjectType = "Header"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced

// RateLimitPolicy 声明限流策略
type RateLimitPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RateLimitPolicySpec `json:"spec,omitempty"`
	Status PolicyStatus        `json:"status,omitempty"`
}

// RateLimitPolicyList 表示 RateLimitPolicy 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RateLimitPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RateLimitPolicy `json:"items"`
}

// RateLimitPolicySpec 定义限流策略配置
type RateLimitPolicySpec struct {
	// DisplayName 保存控制台展示名称，不参与策略匹配
	DisplayName string `json:"displayName"`
	// Enabled 为 false 时保留策略但不执行限流
	Enabled bool `json:"enabled"`
	// TargetRefs 为空时策略保存为未应用状态
	// +listType=map
	// +listMapKey=kind
	// +listMapKey=name
	TargetRefs []PolicyTargetRef `json:"targetRefs,omitempty"`
	// Subject 决定目标内哪些请求共享同一计数器
	Subject RateLimitSubject `json:"subject"`
	// Limit 定义单个计数器的请求上限和时间窗口
	Limit RateLimit `json:"limit"`
}

// RateLimitSubject 定义目标内共享额度的请求主体
type RateLimitSubject struct {
	// Type 选择全部请求、客户端 IP 或 Header 值作为计数主体
	Type RateLimitSubjectType `json:"type"`
	// HeaderName 只在 Header 主体下使用
	HeaderName string `json:"headerName,omitempty"`
}

// RateLimit 定义指定时间窗口内允许的请求数
type RateLimit struct {
	// Requests 是每个时间窗口允许的最大请求数
	Requests int64 `json:"requests"`
	// WindowSeconds 是固定计数窗口的秒数
	WindowSeconds int64 `json:"windowSeconds"`
}

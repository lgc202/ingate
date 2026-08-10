package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	// RateLimitMaxRequests 是数据面单条限流额度支持的最大请求数
	RateLimitMaxRequests int64 = 1<<31 - 1
	// RateLimitMaxWindowSeconds 是数据面单条限流窗口支持的最大秒数
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
	DisplayName string `json:"displayName"`
	Enabled     bool   `json:"enabled"`
	// +listType=map
	// +listMapKey=kind
	// +listMapKey=name
	TargetRefs []PolicyTargetRef `json:"targetRefs,omitempty"`
	Subject    RateLimitSubject  `json:"subject"`
	Limit      RateLimit         `json:"limit"`
}

// RateLimitSubject 定义目标内共享额度的请求主体
type RateLimitSubject struct {
	Type       RateLimitSubjectType `json:"type"`
	HeaderName string               `json:"headerName,omitempty"`
}

// RateLimit 定义指定时间窗口内允许的请求数
type RateLimit struct {
	Requests      int64 `json:"requests"`
	WindowSeconds int64 `json:"windowSeconds"`
}

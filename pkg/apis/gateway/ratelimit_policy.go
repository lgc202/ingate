package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
	// +listType=atomic
	TargetRefs []PolicyTargetRef `json:"targetRefs,omitempty"`
	// +listType=atomic
	Rules         []RateLimitRule        `json:"rules"`
	Response      RateLimitResponse      `json:"response,omitempty"`
	FailurePolicy RateLimitFailurePolicy `json:"failurePolicy,omitempty"`
}

// RateLimitRule 定义一条限流规则
type RateLimitRule struct {
	Name  string         `json:"name"`
	Key   RateLimitKey   `json:"key"`
	Limit RateLimitQuota `json:"limit"`
}

// RateLimitKey 定义限流计数 key
type RateLimitKey struct {
	// +listType=atomic
	Parts []RateLimitKeyPart `json:"parts"`
}

// RateLimitKeyPart 定义限流 key 的一个组成部分
type RateLimitKeyPart struct {
	Type RateLimitKeyType `json:"type"`
	Name string           `json:"name,omitempty"`
}

// RateLimitQuota 定义限流额度
type RateLimitQuota struct {
	Requests      int `json:"requests"`
	WindowSeconds int `json:"windowSeconds"`
	// Burst 表示令牌桶容量，0 表示使用 Requests 作为容量
	Burst int `json:"burst,omitempty"`
}

// RateLimitResponse 定义超限响应
type RateLimitResponse struct {
	StatusCode         int    `json:"statusCode,omitempty"`
	Message            string `json:"message,omitempty"`
	QuotaHeaderEnabled bool   `json:"quotaHeaderEnabled,omitempty"`
}

package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// RateLimitPolicy 声明限流策略
type RateLimitPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RateLimitPolicySpec `json:"spec,omitempty"`
	Status ResourceStatus      `json:"status,omitempty"`
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
	DisplayName string        `json:"displayName"`
	Description string        `json:"description,omitempty"`
	Enabled     bool          `json:"enabled"`
	Mode        RateLimitMode `json:"mode"`
	// +listType=atomic
	Rules         []RateLimitRule        `json:"rules"`
	Global        *GlobalRateLimitConfig `json:"global,omitempty"`
	Response      RateLimitResponse      `json:"response,omitempty"`
	FailurePolicy RateLimitFailurePolicy `json:"failurePolicy,omitempty"`
}

// RateLimitRule 定义一条限流规则
type RateLimitRule struct {
	Name      string             `json:"name"`
	Key       RateLimitKey       `json:"key"`
	Limit     RateLimitQuota     `json:"limit"`
	Algorithm RateLimitAlgorithm `json:"algorithm,omitempty"`
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
	Burst         int `json:"burst,omitempty"`
}

// GlobalRateLimitConfig 定义 Redis-backed global limit 配置
type GlobalRateLimitConfig struct {
	RedisRef      string `json:"redisRef"`
	Prefix        string `json:"prefix,omitempty"`
	TimeoutMillis int    `json:"timeoutMillis,omitempty"`
}

// RateLimitResponse 定义超限响应
type RateLimitResponse struct {
	StatusCode         int    `json:"statusCode,omitempty"`
	Message            string `json:"message,omitempty"`
	QuotaHeaderEnabled bool   `json:"quotaHeaderEnabled,omitempty"`
}

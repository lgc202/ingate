package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// TokenQuotaPeriod 表示自然日、自然周或自然月额度周期。
type TokenQuotaPeriod string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

const (
	// TokenQuotaPeriodDay 按策略时区的自然日统计。
	TokenQuotaPeriodDay TokenQuotaPeriod = "Day"
	// TokenQuotaPeriodWeek 按策略时区从周一开始的自然周统计。
	TokenQuotaPeriodWeek TokenQuotaPeriod = "Week"
	// TokenQuotaPeriodMonth 按策略时区的自然月统计。
	TokenQuotaPeriodMonth TokenQuotaPeriod = "Month"
)

// TokenQuotaPolicy 声明调用方可使用的模型 Token 额度。
type TokenQuotaPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TokenQuotaPolicySpec `json:"spec,omitempty"`
	Status PolicyStatus         `json:"status,omitempty"`
}

// TokenQuotaPolicyList 表示 TokenQuotaPolicy 资源列表。
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TokenQuotaPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []TokenQuotaPolicy `json:"items"`
}

// TokenQuotaPolicySpec 定义调用方额度及其自然周期。
type TokenQuotaPolicySpec struct {
	// DisplayName 保存控制台展示名称，不参与额度匹配。
	DisplayName string `json:"displayName"`
	// Enabled 为 false 时保留配置但不检查和结算额度。
	Enabled bool `json:"enabled"`
	// TargetRefs 只允许引用 Caller；同一策略中的每个 Caller 独立计数。
	// +listType=map
	// +listMapKey=kind
	// +listMapKey=name
	TargetRefs []PolicyTargetRef `json:"targetRefs,omitempty"`
	// TimeZone 使用 IANA 时区确定自然周期边界。
	TimeZone string `json:"timeZone"`
	// Limits 可同时配置日、周和月额度，每种周期最多一项。
	// +listType=map
	// +listMapKey=period
	Limits []TokenQuotaLimit `json:"limits"`
}

// TokenQuotaLimit 定义一个自然周期内允许使用的总 Token 数。
type TokenQuotaLimit struct {
	// Period 指定自然日、自然周或自然月。
	Period TokenQuotaPeriod `json:"period"`
	// Tokens 是该周期内输入和输出 Token 的总上限。
	Tokens int64 `json:"tokens"`
}

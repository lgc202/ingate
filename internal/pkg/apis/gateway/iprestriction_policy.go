package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IPRestrictionPolicy 声明客户端 IP 访问限制策略。
type IPRestrictionPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPRestrictionPolicySpec `json:"spec,omitempty"`
	Status PolicyStatus            `json:"status,omitempty"`
}

// IPRestrictionPolicyList 表示 IPRestrictionPolicy 资源列表。
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type IPRestrictionPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []IPRestrictionPolicy `json:"items"`
}

// IPRestrictionPolicySpec 定义客户端 IP 允许列表或拒绝列表。
type IPRestrictionPolicySpec struct {
	// DisplayName 保存控制台展示名称，不参与策略匹配。
	DisplayName string `json:"displayName"`
	// Enabled 为 false 时保留策略但不执行 IP 检查。
	Enabled bool `json:"enabled"`
	// TargetRefs 为空时策略保存为未应用状态。
	// +listType=map
	// +listMapKey=kind
	// +listMapKey=name
	TargetRefs []PolicyTargetRef `json:"targetRefs,omitempty"`
	// Allow 与 Deny 必须且只能配置其中一个，每项是 IP 或 CIDR。
	// +listType=set
	Allow []string `json:"allow,omitempty"`
	// Deny 与 Allow 必须且只能配置其中一个，每项是 IP 或 CIDR。
	// +listType=set
	Deny []string `json:"deny,omitempty"`
}

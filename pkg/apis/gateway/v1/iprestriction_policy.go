package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced

// IPRestrictionPolicy 声明客户端 IP 访问限制策略
type IPRestrictionPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPRestrictionPolicySpec `json:"spec,omitempty"`
	Status PolicyStatus            `json:"status,omitempty"`
}

// IPRestrictionPolicyList 表示 IPRestrictionPolicy 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type IPRestrictionPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []IPRestrictionPolicy `json:"items"`
}

// IPRestrictionPolicySpec 定义客户端 IP 允许列表或拒绝列表
type IPRestrictionPolicySpec struct {
	DisplayName string `json:"displayName"`
	Enabled     bool   `json:"enabled"`
	// +listType=map
	// +listMapKey=kind
	// +listMapKey=name
	TargetRefs []PolicyTargetRef `json:"targetRefs,omitempty"`
	// +listType=set
	Allow []string `json:"allow,omitempty"`
	// +listType=set
	Deny []string `json:"deny,omitempty"`
}

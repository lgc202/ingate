package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced

// PolicyBinding 声明一组策略绑定到哪个资源
type PolicyBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicyBindingSpec `json:"spec,omitempty"`
	Status ResourceStatus    `json:"status,omitempty"`
}

// PolicyBindingList 表示 PolicyBinding 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PolicyBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PolicyBinding `json:"items"`
}

// PolicyBindingSpec 定义策略绑定目标和策略引用
type PolicyBindingSpec struct {
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	Enabled     bool            `json:"enabled"`
	TargetRef   PolicyTargetRef `json:"targetRef"`
	// +listType=atomic
	Policies []PolicyRef `json:"policies"`
}

// PolicyTargetRef 表示策略绑定目标资源，当前执行链路只支持 Gateway 和 Route
type PolicyTargetRef struct {
	Kind     Kind   `json:"kind"`
	Name     string `json:"name"`
	RuleName string `json:"ruleName,omitempty"`
}

// PolicyRef 表示被绑定的策略资源引用
type PolicyRef struct {
	Kind Kind   `json:"kind"`
	Name string `json:"name"`
}

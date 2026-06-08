package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced

// RuntimeGroup 表示一组数据面运行时的逻辑投放单元
type RuntimeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RuntimeGroupSpec `json:"spec,omitempty"`
	Status ResourceStatus   `json:"status,omitempty"`
}

// RuntimeGroupList 表示 RuntimeGroup 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RuntimeGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RuntimeGroup `json:"items"`
}

// RuntimeGroupSpec 定义一组数据面运行时的 target 和控制台展示信息
type RuntimeGroupSpec struct {
	// DisplayName 保存控制台展示名称，不参与引用匹配
	DisplayName string `json:"displayName,omitempty"`
	// Description 保存运维识别用的说明，不参与运行时匹配
	Description string `json:"description,omitempty"`
	// Enabled 表示该运行组是否允许承载新的 Gateway 配置
	Enabled bool `json:"enabled"`
	// TargetRef 表示该运行组对应的运行时 target
	TargetRef TargetRef `json:"targetRef"`
}

// TargetRef 引用一个运行时 target
type TargetRef struct {
	Name string `json:"name"`
}

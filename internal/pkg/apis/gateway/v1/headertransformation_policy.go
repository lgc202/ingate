package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// HeaderTransformationOperation 表示请求或响应 Header 的修改动作。
type HeaderTransformationOperation string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced

const (
	// HeaderTransformationRemove 删除指定 Header。
	HeaderTransformationRemove HeaderTransformationOperation = "Remove"
	// HeaderTransformationRename 重命名指定 Header。
	HeaderTransformationRename HeaderTransformationOperation = "Rename"
	// HeaderTransformationReplace 替换已存在 Header 的值。
	HeaderTransformationReplace HeaderTransformationOperation = "Replace"
	// HeaderTransformationAdd 在 Header 不存在时添加值。
	HeaderTransformationAdd HeaderTransformationOperation = "Add"
	// HeaderTransformationAppend 向 Header 追加值。
	HeaderTransformationAppend HeaderTransformationOperation = "Append"
)

// HeaderTransformationPolicy 声明请求和响应 Header 转换策略。
type HeaderTransformationPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HeaderTransformationPolicySpec `json:"spec,omitempty"`
	Status PolicyStatus                   `json:"status,omitempty"`
}

// HeaderTransformationPolicyList 表示 HeaderTransformationPolicy 资源列表。
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type HeaderTransformationPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []HeaderTransformationPolicy `json:"items"`
}

// HeaderTransformationPolicySpec 定义转换规则及其作用路由。
type HeaderTransformationPolicySpec struct {
	// DisplayName 保存控制台展示名称，不参与流量匹配。
	DisplayName string `json:"displayName"`
	// Enabled 为 false 时保留策略但不修改请求和响应。
	Enabled bool `json:"enabled"`
	// TargetRefs 为空时策略保存为未应用状态，当前只允许引用 Route。
	// +listType=map
	// +listMapKey=kind
	// +listMapKey=name
	TargetRefs []PolicyTargetRef `json:"targetRefs,omitempty"`
	// RequestRules 按声明顺序修改发往上游服务的请求 Header。
	// +listType=atomic
	RequestRules []HeaderTransformationRule `json:"requestRules,omitempty"`
	// ResponseRules 按声明顺序修改返回给客户端的响应 Header。
	// +listType=atomic
	ResponseRules []HeaderTransformationRule `json:"responseRules,omitempty"`
}

// HeaderTransformationRule 表示一条 Header 修改规则。
type HeaderTransformationRule struct {
	// Operation 决定删除、重命名、替换、添加或追加 Header。
	Operation HeaderTransformationOperation `json:"operation"`
	// Name 是待修改的 Header 名称。
	Name string `json:"name"`
	// Value 是 Rename 的新名称或其他写操作的目标值；Remove 不使用该字段。
	Value string `json:"value,omitempty"`
}

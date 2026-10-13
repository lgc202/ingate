package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +resourceName=mcproutes
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MCPRoute 将 Gateway 的 MCP 入口映射到单个 Backend，不声明认证或聚合策略。
type MCPRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPRouteSpec   `json:"spec"`
	Status MCPRouteStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MCPRouteList 表示 MCPRoute 集合。
type MCPRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []MCPRoute `json:"items"`
}

// MCPRouteSpec 定义 MCP 入口的父引用、路径和单个后端。
type MCPRouteSpec struct {
	// DisplayName 是可修改且允许重复的展示名称，不作为资源引用依据。
	DisplayName string `json:"displayName,omitempty"`
	// +listType=atomic
	ParentRefs []ParentReference `json:"parentRefs"`
	// Path 是入口绝对路径；省略时默认 /mcp，显式空值无效。
	Path *string `json:"path,omitempty"`
	// +listType=atomic
	BackendRefs []MCPBackendReference `json:"backendRefs"`
}

// MCPBackendReference 引用集群级 Backend，并声明上游 MCP 路径。
type MCPBackendReference struct {
	Name string `json:"name"`
	// Path 是上游绝对路径；省略时默认 /mcp，保留显式路径和尾斜杠。
	Path *string `json:"path,omitempty"`
}

// MCPRouteStatus 保存协调器的父级观测结果，不由资源写入操作填充。
type MCPRouteStatus struct {
	// +listType=atomic
	Parents []RouteParentStatus `json:"parents,omitempty"`
}

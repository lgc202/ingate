package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HTTPRoute 将 Gateway 的 HTTP 入口匹配到 Backend。
type HTTPRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HTTPRouteSpec   `json:"spec"`
	Status HTTPRouteStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HTTPRouteList 表示 HTTPRoute 集合。
type HTTPRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []HTTPRoute `json:"items"`
}

// HTTPRouteSpec 定义 HTTPRoute 的期望状态。
type HTTPRouteSpec struct {
	// DisplayName 是可修改且允许重复的展示名称，不作为资源引用依据。
	DisplayName string `json:"displayName,omitempty"`
	// +listType=atomic
	ParentRefs []ParentReference `json:"parentRefs"`
	// +listType=atomic
	Rules []HTTPRouteRule `json:"rules"`
}

// ParentReference 引用 Gateway；省略 SectionName 时附着所有受支持的 HTTP Listener。
type ParentReference struct {
	Name        string `json:"name"`
	SectionName string `json:"sectionName,omitempty"`
}

// HTTPRouteRule 包含 OR 匹配条件和单个 Backend 引用；Name 可选且不参与排序。
type HTTPRouteRule struct {
	Name string `json:"name,omitempty"`
	// +listType=atomic
	Matches []HTTPRouteMatch `json:"matches,omitempty"`
	// +listType=atomic
	BackendRefs []BackendReference `json:"backendRefs"`
}

// HTTPRouteMatch 是单个请求匹配条件；省略 Path 时匹配 PathPrefix /。
type HTTPRouteMatch struct {
	Path *PathMatch `json:"path,omitempty"`
}

// BackendReference 引用当前集群作用域内的 Backend。
type BackendReference struct {
	Name string `json:"name"`
}

// PathMatch 定义请求路径的匹配方式和值。
type PathMatch struct {
	// Type 选择完整路径或路径前缀匹配。
	Type PathMatchType `json:"type,omitempty"`
	// Value 是不含查询参数和片段的绝对请求路径；PathPrefix 匹配时忽略尾斜杠，根路径除外。
	Value string `json:"value,omitempty"`
}

// PathMatchType 表示请求路径的匹配方式。
type PathMatchType string

const (
	// PathMatchExact 匹配完整的请求路径。
	PathMatchExact PathMatchType = "Exact"
	// PathMatchPrefix 按路径段匹配前缀；/api 不匹配 /apix。
	PathMatchPrefix PathMatchType = "PathPrefix"
)

// HTTPRouteStatus 保存 Controller 对 HTTPRoute 的观测结果。
type HTTPRouteStatus struct {
	// +listType=atomic
	Parents []RouteParentStatus `json:"parents,omitempty"`
}

// RouteParentStatus 沿用 Gateway API 的父引用、控制器和 Conditions 形状。
type RouteParentStatus struct {
	ParentRef      ParentReference `json:"parentRef"`
	ControllerName string          `json:"controllerName"`
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

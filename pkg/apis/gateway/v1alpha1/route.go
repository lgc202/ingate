package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Route 将 Gateway 的一个监听入口上的请求转发到 Upstream。
type Route struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RouteSpec   `json:"spec"`
	Status RouteStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RouteList 表示 Route 集合。
type RouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Route `json:"items"`
}

// RouteSpec 定义 Route 的期望状态。
type RouteSpec struct {
	// GatewayName 引用此路由所属的 Gateway。
	GatewayName string `json:"gatewayName"`
	// ListenerName 引用 Gateway 内的监听入口。
	ListenerName string `json:"listenerName"`
	// Path 定义请求路径的匹配条件。
	Path PathMatch `json:"path"`
	// UpstreamName 引用接收请求的后端资源。
	UpstreamName string `json:"upstreamName"`
}

// PathMatch 定义请求路径的匹配方式和值。
type PathMatch struct {
	// Type 选择完整路径或路径前缀匹配。
	Type PathMatchType `json:"type"`
	// Value 是不含查询参数和片段的绝对请求路径；PathPrefix 匹配时忽略尾斜杠，根路径除外。
	Value string `json:"value"`
}

// PathMatchType 表示请求路径的匹配方式。
type PathMatchType string

const (
	// PathMatchExact 匹配完整的请求路径。
	PathMatchExact PathMatchType = "Exact"
	// PathMatchPrefix 按路径段匹配前缀；/api 不匹配 /apix。
	PathMatchPrefix PathMatchType = "PathPrefix"
)

// RouteStatus 保存 Controller 对 Route 的观测结果。
type RouteStatus struct {
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

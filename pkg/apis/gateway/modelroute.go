package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModelRoute 将 Gateway 的模型请求匹配到 AIBackend；不声明操作路径或客户端格式。
type ModelRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModelRouteSpec   `json:"spec"`
	Status ModelRouteStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModelRouteList 表示 ModelRoute 集合。
type ModelRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ModelRoute `json:"items"`
}

// ModelRouteSpec 定义模型入口的父引用及有序规则，不保存凭据。
type ModelRouteSpec struct {
	// DisplayName 是可修改且允许重复的展示名称，不作为资源引用依据。
	DisplayName string `json:"displayName,omitempty"`
	// +listType=atomic
	ParentRefs []ParentReference `json:"parentRefs"`
	// +listType=atomic
	Rules []ModelRouteRule `json:"rules"`
}

// ModelRouteRule 包含 OR 匹配条件和单个 AIBackend 引用；Name 可选且不参与排序。
type ModelRouteRule struct {
	Name string `json:"name,omitempty"`
	// +listType=atomic
	Matches []ModelRouteMatch `json:"matches,omitempty"`
	// +listType=atomic
	BackendRefs []AIBackendReference `json:"backendRefs"`
}

// ModelRouteMatch 定义 AND 关系的 Header 条件；空条件表示所有模型。
type ModelRouteMatch struct {
	// +listType=atomic
	Headers []HTTPHeaderMatch `json:"headers,omitempty"`
}

// HTTPHeaderMatch 定义 Header 匹配；ModelRoute 首版只允许匹配网关提取的 x-ai-eg-model。
type HTTPHeaderMatch struct {
	// Name 按 HTTP Header 名的大小写不敏感规则识别，不信任客户端同名 Header。
	Name string `json:"name"`
	// Type 省略或为空时默认 Exact。
	Type HeaderMatchType `json:"type,omitempty"`
	// Value 按原值匹配模型名称，不去除空白或改写大小写。
	Value string `json:"value"`
}

// HeaderMatchType 表示 HTTP Header 的匹配方式。
type HeaderMatchType string

const (
	// HeaderMatchExact 按原值精确匹配 Header 值。
	HeaderMatchExact HeaderMatchType = "Exact"
)

// AIBackendReference 引用同一 Namespace 内的 AIBackend，并可覆盖出站请求的模型名称。
type AIBackendReference struct {
	Name string `json:"name"`
	// ModelNameOverride 省略或为空时保留请求模型名，不从资源名称推断。
	ModelNameOverride string `json:"modelNameOverride,omitempty"`
}

// ModelRouteStatus 保存协调器的父级观测结果，不由资源写入操作填充。
type ModelRouteStatus struct {
	// +listType=atomic
	Parents []RouteParentStatus `json:"parents,omitempty"`
}

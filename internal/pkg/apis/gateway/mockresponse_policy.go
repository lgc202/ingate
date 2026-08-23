package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MockResponsePolicy 声明命中 Route 后由网关直接返回的固定 HTTP 响应
type MockResponsePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MockResponsePolicySpec `json:"spec,omitempty"`
	Status PolicyStatus           `json:"status,omitempty"`
}

// MockResponsePolicyList 表示 MockResponsePolicy 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type MockResponsePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []MockResponsePolicy `json:"items"`
}

// MockResponsePolicySpec 定义固定响应内容及其作用路由
type MockResponsePolicySpec struct {
	// DisplayName 保存控制台展示名称，不参与流量匹配
	DisplayName string `json:"displayName"`
	// Enabled 为 false 时保留策略但继续把请求转发到上游
	Enabled bool `json:"enabled"`
	// TargetRefs 为空时策略保存为未应用状态，当前只允许引用 Route
	// +listType=map
	// +listMapKey=kind
	// +listMapKey=name
	TargetRefs []PolicyTargetRef `json:"targetRefs,omitempty"`
	// StatusCode 是返回给客户端的 HTTP 状态码
	StatusCode int32 `json:"statusCode"`
	// ContentType 是响应正文的媒体类型，同时写入 Content-Type Header
	ContentType string `json:"contentType"`
	// Headers 是除 Content-Type 外的固定响应 Header
	// +listType=map
	// +listMapKey=name
	Headers []HeaderValue `json:"headers,omitempty"`
	// Body 是原样返回给客户端的响应正文
	Body string `json:"body,omitempty"`
}

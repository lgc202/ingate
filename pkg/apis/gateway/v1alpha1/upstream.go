package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Upstream 声明 Route 可引用的一组后端地址。
type Upstream struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UpstreamSpec   `json:"spec"`
	Status UpstreamStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UpstreamList 表示 Upstream 集合。
type UpstreamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Upstream `json:"items"`
}

// UpstreamSpec 定义 Upstream 的期望状态。
type UpstreamSpec struct {
	// +listType=atomic
	Endpoints []Endpoint `json:"endpoints"`
}

// Endpoint 定义一个后端地址及其相对流量权重。
type Endpoint struct {
	// Address 是不包含端口的主机名或 IP 地址。
	Address string `json:"address"`
	// Port 是后端服务的监听端口。
	Port int32 `json:"port"`
	// Weight 是该地址在 Upstream 内的相对流量权重。
	Weight int32 `json:"weight"`
}

// UpstreamStatus 保存 Controller 对 Upstream 的观测结果。
type UpstreamStatus struct {
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

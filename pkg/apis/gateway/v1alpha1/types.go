package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Gateway 声明网关对外提供的一组逻辑监听入口。
type Gateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewaySpec   `json:"spec"`
	Status GatewayStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GatewayList 表示 Gateway 集合。
type GatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Gateway `json:"items"`
}

// GatewaySpec 定义 Gateway 的期望状态。
type GatewaySpec struct {
	// DisplayName 是供人阅读的名称，不作为资源引用或流量匹配依据。
	DisplayName string `json:"displayName,omitempty"`
	// +listType=atomic
	Listeners []Listener `json:"listeners"`
}

// Listener 定义数据面对外暴露的一个入口。
type Listener struct {
	Name     string   `json:"name"`
	Protocol Protocol `json:"protocol"`
	Port     int32    `json:"port"`
	Hostname string   `json:"hostname,omitempty"`
}

// Protocol 表示 Listener 处理的应用层协议。
type Protocol string

const (
	// ProtocolHTTP 表示明文 HTTP 流量。
	ProtocolHTTP Protocol = "HTTP"
)

// GatewayStatus 保存 Controller 对 Gateway 的观测结果。
type GatewayStatus struct {
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

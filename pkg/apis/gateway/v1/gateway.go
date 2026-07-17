package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Protocol 表示网关资源中可声明的流量协议
type Protocol string

const (
	// ProtocolHTTP 表示普通 HTTP 流量
	ProtocolHTTP Protocol = "HTTP"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced

// Gateway 声明一个流量入口
type Gateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewaySpec    `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// GatewayList 表示 Gateway 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type GatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Gateway `json:"items"`
}

// GatewaySpec 定义 Gateway 的入口监听和域名绑定
type GatewaySpec struct {
	// DisplayName 保存控制台展示名称，不参与引用和运行时匹配
	DisplayName string `json:"displayName,omitempty"`
	// Description 保存控制台展示和运维识别用的说明，不参与运行时匹配
	Description string `json:"description,omitempty"`
	// Enabled 表示 Gateway 是否参与编译和下发
	Enabled bool `json:"enabled"`
	// +listType=atomic
	Listeners []Listener `json:"listeners"`
	// +listType=atomic
	HostBindings []HostBinding `json:"hostBindings,omitempty"`
}

// Listener 声明一个 Gateway 监听端口
type Listener struct {
	Name     string   `json:"name"`
	Protocol Protocol `json:"protocol"`
	Port     int      `json:"port"`
}

// HostBinding 声明 Host 到 Gateway 监听器的绑定关系
type HostBinding struct {
	Hostname string `json:"hostname,omitempty"`
	// +listType=atomic
	ListenerRefs []string `json:"listenerRefs,omitempty"`
}

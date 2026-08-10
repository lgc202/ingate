package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Protocol 表示网关资源中可声明的流量协议
type Protocol string

const (
	// ProtocolHTTP 表示普通 HTTP 流量
	ProtocolHTTP Protocol = "HTTP"
	// ProtocolHTTPS 表示由 Envoy 终止 TLS 的 HTTPS 流量
	ProtocolHTTPS Protocol = "HTTPS"
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

// GatewaySpec 定义 Gateway 的期望入口配置
type GatewaySpec struct {
	// DisplayName 保存控制台展示名称，不参与引用和运行时匹配
	DisplayName string `json:"displayName,omitempty"`
	// Enabled 表示 Gateway 是否参与编译和下发
	Enabled bool `json:"enabled"`
	// +listType=atomic
	Listeners []Listener `json:"listeners"`
}

// Listener 声明一个 Gateway 对外提供的流量入口
type Listener struct {
	Name           string   `json:"name"`
	Protocol       Protocol `json:"protocol"`
	Port           int      `json:"port"`
	Hostname       string   `json:"hostname,omitempty"`
	CertificateRef string   `json:"certificateRef,omitempty"`
}

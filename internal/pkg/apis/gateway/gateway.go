package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Protocol 表示网关资源中可声明的流量协议。
type Protocol string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

const (
	// ProtocolHTTP 表示普通 HTTP 流量。
	ProtocolHTTP Protocol = "HTTP"
	// ProtocolHTTPS 表示由 Envoy 终止 TLS 的 HTTPS 流量。
	ProtocolHTTPS Protocol = "HTTPS"
)

// Gateway 声明一个流量入口。
type Gateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewaySpec    `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// GatewayList 表示 Gateway 资源列表。
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type GatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Gateway `json:"items"`
}

// GatewaySpec 定义 Gateway 的期望入口配置。
type GatewaySpec struct {
	// DisplayName 保存控制台展示名称，不参与引用和运行时匹配。
	DisplayName string `json:"displayName,omitempty"`
	// Enabled 表示 Gateway 是否参与编译和下发。
	Enabled bool `json:"enabled"`
	// +listType=atomic
	Listeners []Listener `json:"listeners"`
}

// Listener 声明一个 Gateway 对外提供的流量入口。
type Listener struct {
	// Name 在当前 Gateway 内唯一，用于识别监听入口。
	Name string `json:"name"`
	// Protocol 决定是否由 Envoy 终止 TLS。
	Protocol Protocol `json:"protocol"`
	// Port 是 Envoy 对外监听的 TCP 端口。
	Port int `json:"port"`
	// Hostname 为空时接受任意 Host，否则只接受指定域名。
	Hostname string `json:"hostname,omitempty"`
	// CertificateRef 只在 HTTPS 下使用，引用 Certificate 的 metadata.name。
	CertificateRef string `json:"certificateRef,omitempty"`
}

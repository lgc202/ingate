package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Backend 声明 HTTPRoute 可引用的一组后端地址。
type Backend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackendSpec   `json:"spec"`
	Status BackendStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackendList 表示 Backend 集合。
type BackendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Backend `json:"items"`
}

// BackendSpec 定义 Backend 的期望状态。
type BackendSpec struct {
	// DisplayName 是可修改且允许重复的展示名称，不作为资源引用依据。
	DisplayName string `json:"displayName,omitempty"`
	// Endpoints 中的顺序不影响转发；同一地址和端口只能出现一次。
	// +listType=atomic
	Endpoints []Endpoint `json:"endpoints"`
	// TLS 要求加密连接；省略表示明文 HTTP，不从请求推导 TLS 身份。
	TLS *BackendTLSSettings `json:"tls,omitempty"`
}

// BackendTLSSettings 定义一个 Backend 全部端点共享的服务器身份与信任根。
type BackendTLSSettings struct {
	// SNI 是固定 DNS 身份，同时用于 TLS SNI 和证书 DNS SAN 验证。
	SNI string `json:"sni"`
	// WellKnownCACertificates 首版仅接受 System，使用 Envoy 运行环境的系统根证书。
	WellKnownCACertificates string `json:"wellKnownCACertificates"`
}

// Endpoint 定义一个后端地址及其相对流量权重。
type Endpoint struct {
	// Address 是不包含端口的主机名或 IP 地址。
	Address string `json:"address"`
	// Port 是后端服务的监听端口。
	Port int32 `json:"port"`
	// Weight 是该地址在 Backend 内的相对流量权重。
	Weight int32 `json:"weight"`
}

// BackendStatus 保存 Controller 对 Backend 的观测结果。
type BackendStatus struct {
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

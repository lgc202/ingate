package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// LoadBalancingPolicy 表示 Upstream 端点的负载均衡策略
type LoadBalancingPolicy string

const (
	// LoadBalancingRoundRobin 表示轮询
	LoadBalancingRoundRobin LoadBalancingPolicy = "RoundRobin"
	// LoadBalancingLeastRequest 表示最少请求
	LoadBalancingLeastRequest LoadBalancingPolicy = "LeastRequest"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced

// Upstream 声明一个逻辑上游服务
type Upstream struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UpstreamSpec   `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// UpstreamList 表示 Upstream 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UpstreamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Upstream `json:"items"`
}

// UpstreamSpec 定义 HTTP 上游服务的连接方式和端点集合
type UpstreamSpec struct {
	// DisplayName 保存控制台展示名称，不参与资源引用
	DisplayName string `json:"displayName,omitempty"`
	// Endpoints 是当前服务可接收流量的网络端点
	// +listType=atomic
	Endpoints []Endpoint `json:"endpoints"`
	// TLS 描述访问 Upstream 时的服务端身份校验，未配置时使用明文 HTTP
	TLS *UpstreamTLS `json:"tls,omitempty"`
	// LoadBalancing 指定多个端点之间的负载均衡策略
	LoadBalancing LoadBalancingPolicy `json:"loadBalancing,omitempty"`
	// HealthCheck 描述可选的 HTTP 主动健康检查，对象存在即启用
	HealthCheck *UpstreamHealthCheck `json:"healthCheck,omitempty"`
}

// Endpoint 表示 Upstream 的一个网络地址及其相对容量
type Endpoint struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
	// Weight 默认为 1，多个端点之间按相对权重分配流量
	Weight int `json:"weight,omitempty"`
}

// UpstreamTLS 声明使用系统 CA 根证书包校验上游证书
type UpstreamTLS struct {
	// ServerName 用于 TLS SNI 和服务端证书身份校验
	ServerName string `json:"serverName"`
}

// UpstreamHealthCheck 声明 Upstream 的 HTTP 主动健康检查
type UpstreamHealthCheck struct {
	Path            string `json:"path"`
	IntervalSeconds int    `json:"intervalSeconds,omitempty"`
	TimeoutSeconds  int    `json:"timeoutSeconds,omitempty"`
}

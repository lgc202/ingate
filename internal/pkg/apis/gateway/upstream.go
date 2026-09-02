package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// LoadBalancingPolicy 表示 Upstream 端点的负载均衡策略。
type LoadBalancingPolicy string

const (
	// LoadBalancingRoundRobin 表示轮询。
	LoadBalancingRoundRobin LoadBalancingPolicy = "RoundRobin"
	// LoadBalancingLeastRequest 表示最少请求。
	LoadBalancingLeastRequest LoadBalancingPolicy = "LeastRequest"
)

// ModelProtocol 表示模型服务实际提供的 HTTP API 协议。
type ModelProtocol string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

const (
	// ModelProtocolOpenAI 表示 OpenAI Chat Completions 兼容协议。
	ModelProtocolOpenAI ModelProtocol = "OpenAI"
	// ModelProtocolAnthropic 表示 Anthropic Messages 协议。
	ModelProtocolAnthropic ModelProtocol = "Anthropic"
)

// Upstream 声明一个逻辑上游服务。
type Upstream struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UpstreamSpec   `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// UpstreamList 表示 Upstream 资源列表。
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UpstreamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Upstream `json:"items"`
}

// UpstreamSpec 定义 HTTP 上游服务的连接方式和端点集合。
type UpstreamSpec struct {
	// DisplayName 保存控制台展示名称，不参与资源引用。
	DisplayName string `json:"displayName,omitempty"`
	// Endpoints 是当前服务可接收流量的网络端点。
	// +listType=atomic
	Endpoints []Endpoint `json:"endpoints"`
	// TLS 描述访问 Upstream 时的服务端身份校验，未配置时使用明文 HTTP。
	TLS *UpstreamTLS `json:"tls,omitempty"`
	// LoadBalancing 指定多个端点之间的负载均衡策略。
	LoadBalancing LoadBalancingPolicy `json:"loadBalancing,omitempty"`
	// HealthCheck 描述可选的 HTTP 主动健康检查，对象存在即启用。
	HealthCheck *UpstreamHealthCheck `json:"healthCheck,omitempty"`
	// Model 存在时表示当前 Upstream 是模型服务，协议转换由 Ingate 数据面完成。
	Model *ModelUpstream `json:"model,omitempty"`
}

// ModelUpstream 定义模型服务与 Ingate 交互使用的协议。
// 真实模型名属于 Route 的模型映射，同一个模型服务可以承载多个模型。
type ModelUpstream struct {
	// Protocol 决定凭据注入和请求响应转换规则。
	Protocol ModelProtocol `json:"protocol"`
	// APIKey 保存模型服务的访问凭据；Controller 不得把该值写入 Envoy xDS。
	APIKey string `json:"apiKey,omitempty"`
}

// Endpoint 表示 Upstream 的一个网络地址及其相对容量。
type Endpoint struct {
	// Address 是 IP 地址或 DNS 主机名，不包含协议和端口。
	Address string `json:"address"`
	// Port 是服务接收流量的 TCP 端口。
	Port int `json:"port"`
	// Weight 默认为 1，多个端点之间按相对权重分配流量。
	Weight int `json:"weight,omitempty"`
}

// UpstreamTLS 声明使用系统 CA 根证书包校验上游证书。
type UpstreamTLS struct {
	// ServerName 用于 TLS SNI 和服务端证书身份校验。
	ServerName string `json:"serverName"`
}

// UpstreamHealthCheck 声明 Upstream 的 HTTP 主动健康检查。
type UpstreamHealthCheck struct {
	// Path 是主动健康检查请求使用的绝对路径。
	Path string `json:"path"`
	// IntervalSeconds 是相邻两次检查的间隔，默认 10 秒。
	IntervalSeconds int `json:"intervalSeconds,omitempty"`
	// TimeoutSeconds 是单次检查的超时时间，默认 2 秒。
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
}

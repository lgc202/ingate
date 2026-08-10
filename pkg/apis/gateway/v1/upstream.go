package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

// UpstreamType 表示 Upstream 的业务分类
type UpstreamType string

const (
	// UpstreamTypeApplication 表示普通应用服务
	UpstreamTypeApplication UpstreamType = "Application"
	// UpstreamTypeModel 表示模型服务
	UpstreamTypeModel UpstreamType = "Model"
	// UpstreamTypeAgent 表示 Agent 服务
	UpstreamTypeAgent UpstreamType = "Agent"
	// UpstreamTypeMCP 表示 MCP 服务
	UpstreamTypeMCP UpstreamType = "MCP"
)

// ModelProvider 表示模型服务所属的厂商或兼容实现
type ModelProvider string

const (
	// ModelProviderOpenAI 表示 OpenAI 官方服务
	ModelProviderOpenAI ModelProvider = "OpenAI"
	// ModelProviderDeepSeek 表示 DeepSeek 官方服务
	ModelProviderDeepSeek ModelProvider = "DeepSeek"
	// ModelProviderQwen 表示通义千问兼容模式服务
	ModelProviderQwen ModelProvider = "Qwen"
	// ModelProviderAnthropic 表示 Anthropic 官方服务
	ModelProviderAnthropic ModelProvider = "Anthropic"
	// ModelProviderGemini 表示 Gemini 官方服务
	ModelProviderGemini ModelProvider = "Gemini"
	// ModelProviderCustom 表示自定义 OpenAI-compatible 服务
	ModelProviderCustom ModelProvider = "Custom"
)

// ModelProtocol 表示模型厂商使用的请求协议
type ModelProtocol string

const (
	// ModelProtocolOpenAI 表示 OpenAI-compatible 协议
	ModelProtocolOpenAI ModelProtocol = "OpenAI"
	// ModelProtocolAnthropic 表示 Anthropic 原生消息协议
	ModelProtocolAnthropic ModelProtocol = "Anthropic"
	// ModelProtocolGemini 表示 Gemini 原生生成内容协议
	ModelProtocolGemini ModelProtocol = "Gemini"
)

// Protocol 返回模型厂商对应的请求协议
func (p ModelProvider) Protocol() (ModelProtocol, bool) {
	protocol, ok := resource.ModelProvider(p).Protocol()
	return ModelProtocol(protocol), ok
}

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

// UpstreamSpec 定义 Upstream 的业务分类、连接方式和端点集合
type UpstreamSpec struct {
	// DisplayName 保存控制台展示名称，不参与资源引用
	DisplayName string `json:"displayName,omitempty"`
	// Type 用于区分普通应用、模型、MCP 和 Agent 服务
	Type UpstreamType `json:"type"`
	// Endpoints 是当前服务可接收流量的网络端点
	// +listType=atomic
	Endpoints []Endpoint `json:"endpoints"`
	// TLS 描述访问 Upstream 时的服务端身份校验，未配置时使用明文 HTTP
	TLS *UpstreamTLS `json:"tls,omitempty"`
	// LoadBalancing 指定多个端点之间的负载均衡策略
	LoadBalancing LoadBalancingPolicy `json:"loadBalancing,omitempty"`
	// HealthCheck 描述可选的 HTTP 主动健康检查，对象存在即启用
	HealthCheck *UpstreamHealthCheck `json:"healthCheck,omitempty"`
	// Model 只用于模型服务，保存厂商协议参数、模型目录和 API Key
	Model *ModelSpec `json:"model,omitempty"`
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

// ModelSpec 定义模型服务的厂商、API 路径、模型目录和访问密钥
type ModelSpec struct {
	// Provider 决定模型请求协议及认证 Header 规则
	Provider ModelProvider `json:"provider"`
	// BasePath 是追加到模型端点后的厂商 API 基础路径
	BasePath string `json:"basePath"`
	// Models 保存允许 Route 引用的厂商模型名称
	// +listType=atomic
	Models []string `json:"models"`
	// APIKey 保存发送给模型服务的完整 API Key
	APIKey string `json:"apiKey,omitempty"`
}

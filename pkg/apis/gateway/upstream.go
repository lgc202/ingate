package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// UpstreamType 表示 Upstream 的业务分类
type UpstreamType string

const (
	// UpstreamTypeApplication 表示普通应用服务
	UpstreamTypeApplication UpstreamType = "application"
	// UpstreamTypeModel 表示模型服务
	UpstreamTypeModel UpstreamType = "model"
	// UpstreamTypeAgent 表示 Agent 服务
	UpstreamTypeAgent UpstreamType = "agent"
	// UpstreamTypeMCP 表示 MCP 服务
	UpstreamTypeMCP UpstreamType = "mcp"
)

// UpstreamProtocol 表示 Ingate 与 Upstream 之间使用的应用协议
type UpstreamProtocol string

const (
	// UpstreamProtocolHTTP 表示普通 HTTP 服务
	UpstreamProtocolHTTP UpstreamProtocol = "HTTP"
	// UpstreamProtocolOpenAI 表示兼容 OpenAI API 的模型服务
	UpstreamProtocolOpenAI UpstreamProtocol = "OpenAI"
	// UpstreamProtocolAnthropic 表示使用 Anthropic 原生消息协议的模型服务
	UpstreamProtocolAnthropic UpstreamProtocol = "Anthropic"
	// UpstreamProtocolGemini 表示使用 Gemini 原生生成内容协议的模型服务
	UpstreamProtocolGemini UpstreamProtocol = "Gemini"
)

// ModelProvider 表示模型服务所属的厂商或兼容实现
type ModelProvider string

const (
	// ModelProviderOpenAI 表示 OpenAI 官方服务
	ModelProviderOpenAI ModelProvider = "openai"
	// ModelProviderDeepSeek 表示 DeepSeek 官方服务
	ModelProviderDeepSeek ModelProvider = "deepseek"
	// ModelProviderQwen 表示通义千问兼容模式服务
	ModelProviderQwen ModelProvider = "qwen"
	// ModelProviderAnthropic 表示 Anthropic 官方服务
	ModelProviderAnthropic ModelProvider = "anthropic"
	// ModelProviderGemini 表示 Gemini 官方服务
	ModelProviderGemini ModelProvider = "gemini"
	// ModelProviderCustom 表示自定义 OpenAI-compatible 服务
	ModelProviderCustom ModelProvider = "custom"
)

// UpstreamLoadBalancePolicy 表示 Upstream 的负载均衡策略
type UpstreamLoadBalancePolicy string

const (
	// UpstreamLoadBalancePolicyRoundRobin 表示轮询
	UpstreamLoadBalancePolicyRoundRobin UpstreamLoadBalancePolicy = "round_robin"
	// UpstreamLoadBalancePolicyLeastRequest 表示最少请求
	UpstreamLoadBalancePolicyLeastRequest UpstreamLoadBalancePolicy = "least_request"
	// UpstreamLoadBalancePolicyRandom 表示随机
	UpstreamLoadBalancePolicyRandom UpstreamLoadBalancePolicy = "random"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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

// UpstreamSpec 定义 Upstream 的展示信息、流量策略和端点集合
type UpstreamSpec struct {
	// DisplayName 保存控制台展示名称，不参与引用匹配
	DisplayName string `json:"displayName,omitempty"`
	// Type 保存 Upstream 的业务分类，用于区分普通服务、模型服务和 Agent/MCP 服务
	Type UpstreamType `json:"type,omitempty"`
	// Protocol 描述 Ingate 与 Upstream 之间实际使用的应用协议
	Protocol UpstreamProtocol `json:"protocol,omitempty"`
	// TLS 描述访问 Upstream 时的 TLS 配置，未配置时使用明文连接
	TLS *UpstreamTLS `json:"tls,omitempty"`
	// Authentication 描述 Ingate 访问 Upstream 时使用的认证信息
	Authentication *UpstreamAuthentication `json:"authentication,omitempty"`
	// Model 描述模型厂商、API 基础路径和可供路由选择的模型目录
	Model *ModelSpec `json:"model,omitempty"`
	// LoadBalancePolicy 指定多个端点之间的负载均衡策略
	LoadBalancePolicy UpstreamLoadBalancePolicy `json:"loadBalancePolicy,omitempty"`
	// HealthCheck 描述 Upstream 端点的主动健康检查配置
	HealthCheck *UpstreamHealthCheck `json:"healthCheck,omitempty"`
	// +listType=atomic
	Endpoints []Endpoint `json:"endpoints"`
}

// ModelSpec 定义模型服务的厂商协议参数和模型目录
type ModelSpec struct {
	// Provider 表示模型厂商或 OpenAI-compatible 实现类型
	Provider ModelProvider `json:"provider"`
	// APIBasePath 是追加到模型端点后的厂商 API 基础路径
	APIBasePath string `json:"apiBasePath"`
	// +listType=atomic
	Models []ModelCatalogItem `json:"models"`
}

// ModelCatalogItem 表示模型服务对路由开放的一个厂商模型
type ModelCatalogItem struct {
	// Name 是发送给厂商 API 的模型名称，也是路由引用键
	Name string `json:"name"`
	// DisplayName 是控制台展示名称
	DisplayName string `json:"displayName"`
	// Enabled 控制该模型是否允许被模型路由引用
	Enabled bool `json:"enabled"`
}

// UpstreamAuthentication 声明访问 Upstream 时使用的认证配置
type UpstreamAuthentication struct {
	// APIKey 保存静态 API Key 认证信息
	APIKey *APIKeyAuthentication `json:"apiKey,omitempty"`
}

// APIKeyAuthentication 保存静态 API Key
type APIKeyAuthentication struct {
	// Value 保存发送给 Upstream 的完整 API Key
	Value string `json:"value"`
}

// UpstreamTLS 声明访问 Upstream 时使用系统 CA 根证书包校验的 TLS 配置
type UpstreamTLS struct {
	// ServerName 用于 TLS SNI 和服务端证书身份校验
	ServerName string `json:"serverName"`
}

// Endpoint 声明一个上游端点
type Endpoint struct {
	// Name 是端点在 Upstream 内的稳定标识
	Name    string `json:"name,omitempty"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	// Weight 是端点参与负载均衡时的相对权重，取值范围为 1-100
	Weight int `json:"weight,omitempty"`
	// Enabled 控制端点是否参与运行时配置生成
	Enabled bool `json:"enabled"`
}

// UpstreamHealthCheck 声明 Upstream 的主动健康检查配置
type UpstreamHealthCheck struct {
	Enabled         bool   `json:"enabled"`
	Path            string `json:"path,omitempty"`
	IntervalSeconds int    `json:"intervalSeconds,omitempty"`
	TimeoutSeconds  int    `json:"timeoutSeconds,omitempty"`
}

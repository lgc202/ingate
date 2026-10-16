// Package aidataplane 定义 Controller 与 AI Proxy 共享的非敏感编译配置。
// 配置不依赖 Kubernetes API 类型；领域引用由 Controller 提前解析。
// 服务启动参数属于 conf，不进入这里的数据面契约。
package aidataplane

// FormatVersionV1 是当前支持的序列化格式，不是软件版本。
const FormatVersionV1 = "v1"

// Config 是共享数据面的一份完整 AI 配置；空数组表示撤销全部 AI 路由。
// 字段顺序参与内容哈希，修改顺序必须同时评审格式契约与 golden 用例。
// 必填但允许为空的字符串使用 *string：nil 非法，指向空字符串的值合法。
type Config struct {
	FormatVersion string `json:"formatVersion"`
	// Revision 由 Encode 计算；omitempty 仅用于哈希时排除该字段。
	Revision        string           `json:"revision,omitempty"`
	Listeners       []Listener       `json:"listeners"`
	Backends        []Backend        `json:"backends"`
	NetworkBackends []NetworkBackend `json:"networkBackends"`
}

// Listener 汇总一个已接纳端口上的模型与 MCP 主机匹配。
type Listener struct {
	Port  int32  `json:"port"`
	Hosts []Host `json:"hosts"`
}

// Host 是一个 Gateway Listener 的有效主机名及其已排序路由。
type Host struct {
	Name     string  `json:"name"`
	Hostname *string `json:"hostname"`
	// ModelRoutes 与 MCPRoutes 的顺序由 Controller 决定，消费者取首个命中项。
	ModelRoutes []ModelRoute `json:"modelRoutes"`
	MCPRoutes   []MCPRoute   `json:"mcpRoutes"`
}

// ModelRoute 是一条已展开的模型匹配，不再包含领域 parentRefs。
type ModelRoute struct {
	Name    string        `json:"name"`
	Headers []HeaderMatch `json:"headers"`
	// BackendName 必须非 nil；指向空字符串时保留匹配并拒绝，不跳到下一候选。
	BackendName *string `json:"backendName"`
}

// HeaderMatch 匹配 Proxy 从请求中提取的模型，不信任客户端同名 Header。
// 首版只接受 x-ai-eg-model 的 Exact 匹配；空 Headers 表示 catch-all。
type HeaderMatch struct {
	Name  string          `json:"name"`
	Type  HeaderMatchType `json:"type"`
	Value string          `json:"value"`
}

// HeaderMatchType 是模型条件的匹配方式。
type HeaderMatchType string

// HeaderMatchExact 精确匹配提取后的模型名。
const HeaderMatchExact HeaderMatchType = "Exact"

// Backend 是按路由规则生成的协议绑定，不是 AIBackend 领域资源。
type Backend struct {
	Name   string    `json:"name"`
	Schema APISchema `json:"schema"`
	// ModelNameOverride 必须非 nil；指向空字符串时保留请求中的模型名。
	ModelNameOverride  *string `json:"modelNameOverride"`
	NetworkBackendName string  `json:"networkBackendName"`
}

// APISchema 是已默认化的上游协议格式，首版只支持 OpenAI。
type APISchema struct {
	Name   APISchemaName `json:"name"`
	Prefix string        `json:"prefix"`
}

// APISchemaName 是协议绑定的上游 API 格式。
type APISchemaName string

// APISchemaOpenAI 表示 OpenAI 格式，不承诺支持其全部操作。
const APISchemaOpenAI APISchemaName = "OpenAI"

// MCPRoute 关联精确入口路径、网络后端与已默认化的上游路径。
type MCPRoute struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// BackendName 必须非 nil；指向空字符串时保留匹配并拒绝。
	BackendName *string `json:"backendName"`
	BackendPath string  `json:"backendPath"`
}

// NetworkBackend 保存当前配置引用的网络快照，不让 Go 自建端点池。
type NetworkBackend struct {
	Name      string     `json:"name"`
	Endpoints []Endpoint `json:"endpoints"`
	// TLS 省略或 null 表示明文 HTTP；存在时必须包含固定身份和 System 信任根。
	TLS *BackendTLSSettings `json:"tls,omitempty"`
}

// BackendTLSSettings 保存 Envoy 出口的 TLS 身份，不包含证书或凭据。
type BackendTLSSettings struct {
	SNI                     string `json:"sni"`
	WellKnownCACertificates string `json:"wellKnownCACertificates"`
}

// Endpoint 是 Envoy 出口使用的地址、端口和相对权重。
type Endpoint struct {
	Address string `json:"address"`
	Port    int32  `json:"port"`
	Weight  int32  `json:"weight"`
}

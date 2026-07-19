// Package modelprovider 定义 Ingate 内置模型厂商目录及认证元数据
package modelprovider

import "github.com/lgc202/ingate/pkg/llm"

// ID 是模型厂商目录中的稳定标识
type ID string

const (
	// IDOpenAI 表示 OpenAI 官方服务
	IDOpenAI ID = "openai"
	// IDDeepSeek 表示 DeepSeek 官方服务
	IDDeepSeek ID = "deepseek"
	// IDQwen 表示阿里云百炼通义千问兼容模式
	IDQwen ID = "qwen"
	// IDAnthropic 表示 Anthropic Claude 官方服务
	IDAnthropic ID = "anthropic"
	// IDGemini 表示 Google Gemini 官方服务
	IDGemini ID = "gemini"
	// IDCustom 表示用户提供的 OpenAI-compatible 服务
	IDCustom ID = "custom"
)

// Authentication 描述 API Key 如何写入上游 HTTP Header
type Authentication struct {
	Header string
	Prefix string
}

// Definition 描述一个内置厂商的默认连接和认证信息
type Definition struct {
	ID              ID
	DisplayName     string
	Protocol        llm.Protocol
	DefaultBaseURL  string
	DefaultBasePath string
	Authentication  Authentication
	StaticHeaders   map[string]string
}

var catalog = []Definition{
	{
		ID:              IDOpenAI,
		DisplayName:     "OpenAI",
		Protocol:        llm.ProtocolOpenAIChatCompletions,
		DefaultBaseURL:  "https://api.openai.com",
		DefaultBasePath: "/v1",
		Authentication:  Authentication{Header: "Authorization", Prefix: "Bearer "},
	},
	{
		ID:              IDDeepSeek,
		DisplayName:     "DeepSeek",
		Protocol:        llm.ProtocolOpenAIChatCompletions,
		DefaultBaseURL:  "https://api.deepseek.com",
		DefaultBasePath: "/v1",
		Authentication:  Authentication{Header: "Authorization", Prefix: "Bearer "},
	},
	{
		ID:              IDQwen,
		DisplayName:     "通义千问",
		Protocol:        llm.ProtocolOpenAIChatCompletions,
		DefaultBaseURL:  "https://dashscope.aliyuncs.com",
		DefaultBasePath: "/compatible-mode/v1",
		Authentication:  Authentication{Header: "Authorization", Prefix: "Bearer "},
	},
	{
		ID:              IDAnthropic,
		DisplayName:     "Anthropic Claude",
		Protocol:        llm.ProtocolAnthropicMessages,
		DefaultBaseURL:  "https://api.anthropic.com",
		DefaultBasePath: "/v1",
		Authentication:  Authentication{Header: "x-api-key"},
		StaticHeaders:   map[string]string{"anthropic-version": "2023-06-01"},
	},
	{
		ID:              IDGemini,
		DisplayName:     "Google Gemini",
		Protocol:        llm.ProtocolGeminiGenerateContent,
		DefaultBaseURL:  "https://generativelanguage.googleapis.com",
		DefaultBasePath: "/v1beta",
		Authentication:  Authentication{Header: "x-goog-api-key"},
	},
	{
		ID:              IDCustom,
		DisplayName:     "自定义 OpenAI 兼容服务",
		Protocol:        llm.ProtocolOpenAIChatCompletions,
		DefaultBasePath: "/v1",
		Authentication:  Authentication{Header: "Authorization", Prefix: "Bearer "},
	},
}

// Catalog 返回内置厂商目录的独立副本
func Catalog() []Definition {
	definitions := make([]Definition, len(catalog))
	for i, definition := range catalog {
		definitions[i] = cloneDefinition(definition)
	}
	return definitions
}

// Lookup 按稳定标识查找内置厂商定义
func Lookup(id ID) (Definition, bool) {
	for _, definition := range catalog {
		if definition.ID == id {
			return cloneDefinition(definition), true
		}
	}
	return Definition{}, false
}

// ValidAPIKey 检查值能否安全写入单个 HTTP Header
//
// API Key 可以包含空格和非 token68 字符，但不能是空值或包含控制字符
func ValidAPIKey(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] == 0x7f {
			return false
		}
	}
	return true
}

func cloneDefinition(definition Definition) Definition {
	if definition.StaticHeaders == nil {
		return definition
	}
	headers := make(map[string]string, len(definition.StaticHeaders))
	for name, value := range definition.StaticHeaders {
		headers[name] = value
	}
	definition.StaticHeaders = headers
	return definition
}

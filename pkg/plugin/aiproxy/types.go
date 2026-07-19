package aiproxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	// MaxResponseBodyBytes 是普通模型响应在 Wasm 中整体转换时允许的最大缓冲大小
	MaxResponseBodyBytes = 8 << 20
	// ResponseBufferLimitBytes 是 AI Listener 的连接缓冲软限制，预留余量让 Wasm 能观察到普通响应越界
	ResponseBufferLimitBytes = MaxResponseBodyBytes + (1 << 20)
)

// Protocol 表示 AI Proxy 与目标模型服务之间使用的线上协议
type Protocol string

const (
	// ProtocolOpenAI 表示 OpenAI-compatible Chat Completions 协议
	ProtocolOpenAI Protocol = "OpenAI"
	// ProtocolAnthropic 表示 Anthropic Messages 协议
	ProtocolAnthropic Protocol = "Anthropic"
	// ProtocolGemini 表示 Gemini generateContent 协议
	ProtocolGemini Protocol = "Gemini"
)

// PluginConfig 表示真正下发给 AI Proxy Wasm 插件的运行时配置
type PluginConfig struct {
	Routes []RouteConfig `json:"routes"`
}

// RouteConfig 表示一条 RouteRule 的模型路由执行配置
type RouteConfig struct {
	GatewayName string         `json:"gatewayName"`
	RouteName   string         `json:"routeName"`
	RuleName    string         `json:"ruleName"`
	ConfigID    string         `json:"configID"`
	Targets     []TargetConfig `json:"targets"`
	Models      []ModelConfig  `json:"models"`
}

// TargetConfig 表示模型路由可选择的一个实际模型服务
//
// Cluster 和认证信息只能由 Controller 生成，客户端请求中的同名 Header 不可信
type TargetConfig struct {
	ID           string         `json:"id"`
	Provider     string         `json:"provider"`
	Protocol     Protocol       `json:"protocol"`
	Cluster      string         `json:"cluster"`
	BasePath     string         `json:"basePath"`
	APIKey       string         `json:"apiKey,omitempty"`
	APIKeyHeader string         `json:"apiKeyHeader,omitempty"`
	APIKeyPrefix string         `json:"apiKeyPrefix,omitempty"`
	Headers      []HeaderConfig `json:"headers,omitempty"`
}

// HeaderConfig 表示发送给模型服务的固定请求 Header
type HeaderConfig struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ModelConfig 表示客户端模型名称到实际目标和厂商模型名称的可执行映射
type ModelConfig struct {
	Model         string `json:"model"`
	TargetID      string `json:"targetID"`
	UpstreamModel string `json:"upstreamModel"`
}

// ParsePluginConfig 严格解析 Listener 级 AI Proxy 插件配置
func ParsePluginConfig(data []byte) (PluginConfig, error) {
	var cfg PluginConfig
	if err := decodeStrict(data, &cfg); err != nil {
		return PluginConfig{}, err
	}
	if err := cfg.validate(); err != nil {
		return PluginConfig{}, err
	}
	return cfg, nil
}

func (c PluginConfig) validate() error {
	routes := make(map[string]bool, len(c.Routes))
	for i, route := range c.Routes {
		if route.GatewayName == "" {
			return fmt.Errorf("routes[%d].gatewayName must not be empty", i)
		}
		if route.RouteName == "" {
			return fmt.Errorf("routes[%d].routeName must not be empty", i)
		}
		if route.RuleName == "" {
			return fmt.Errorf("routes[%d].ruleName must not be empty", i)
		}
		if route.ConfigID == "" {
			return fmt.Errorf("routes[%d].configID must not be empty", i)
		}
		if len(route.Targets) == 0 {
			return fmt.Errorf("routes[%d].targets must not be empty", i)
		}
		if len(route.Models) == 0 {
			return fmt.Errorf("routes[%d].models must not be empty", i)
		}

		routeKey := route.GatewayName + "\x00" + route.RouteName + "\x00" + route.RuleName
		if routes[routeKey] {
			return fmt.Errorf("routes[%d] duplicates route rule %q/%q/%q", i, route.GatewayName, route.RouteName, route.RuleName)
		}
		routes[routeKey] = true

		targets := make(map[string]bool, len(route.Targets))
		for j, target := range route.Targets {
			if err := target.validate(i, j); err != nil {
				return err
			}
			if targets[target.ID] {
				return fmt.Errorf("routes[%d].targets[%d] duplicates target %q", i, j, target.ID)
			}
			targets[target.ID] = true
		}

		models := make(map[string]bool, len(route.Models))
		for j, model := range route.Models {
			if model.Model == "" {
				return fmt.Errorf("routes[%d].models[%d].model must not be empty", i, j)
			}
			if model.TargetID == "" {
				return fmt.Errorf("routes[%d].models[%d].targetID must not be empty", i, j)
			}
			if !targets[model.TargetID] {
				return fmt.Errorf("routes[%d].models[%d] references missing target %q", i, j, model.TargetID)
			}
			if model.UpstreamModel == "" {
				return fmt.Errorf("routes[%d].models[%d].upstreamModel must not be empty", i, j)
			}
			if models[model.Model] {
				return fmt.Errorf("routes[%d].models[%d] duplicates model %q", i, j, model.Model)
			}
			models[model.Model] = true
		}
	}
	return nil
}

func (c TargetConfig) validate(routeIndex, targetIndex int) error {
	prefix := fmt.Sprintf("routes[%d].targets[%d]", routeIndex, targetIndex)
	if c.ID == "" {
		return fmt.Errorf("%s.id must not be empty", prefix)
	}
	if c.Provider == "" {
		return fmt.Errorf("%s.provider must not be empty", prefix)
	}
	switch c.Protocol {
	case ProtocolOpenAI, ProtocolAnthropic, ProtocolGemini:
	default:
		return fmt.Errorf("%s.protocol %q is not supported", prefix, c.Protocol)
	}
	if c.Cluster == "" {
		return fmt.Errorf("%s.cluster must not be empty", prefix)
	}
	if c.BasePath == "" || !strings.HasPrefix(c.BasePath, "/") {
		return fmt.Errorf("%s.basePath must start with /", prefix)
	}
	if c.APIKey == "" {
		if c.APIKeyHeader != "" || c.APIKeyPrefix != "" {
			return fmt.Errorf("%s API key header or prefix is set without an API key", prefix)
		}
	} else {
		if c.APIKeyHeader == "" {
			return fmt.Errorf("%s.apiKeyHeader must not be empty when API key is configured", prefix)
		}
		if !validHeaderValue(c.APIKeyPrefix + c.APIKey) {
			return fmt.Errorf("%s.apiKey contains unsupported control characters", prefix)
		}
	}
	seenHeaders := make(map[string]bool, len(c.Headers))
	for i, header := range c.Headers {
		name := strings.ToLower(header.Name)
		if name == "" || header.Value == "" || !validHeaderValue(header.Value) {
			return fmt.Errorf("%s.headers[%d] name and value must not be empty", prefix, i)
		}
		if seenHeaders[name] {
			return fmt.Errorf("%s.headers[%d] duplicates header %q", prefix, i, header.Name)
		}
		seenHeaders[name] = true
	}
	return nil
}

func validHeaderValue(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] == 0x7f {
			return false
		}
	}
	return true
}

func decodeStrict(data []byte, value any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errors.New("ai proxy config must be a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("ai proxy config contains multiple JSON values")
		}
		return err
	}
	return nil
}

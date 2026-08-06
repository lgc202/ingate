// Package routeconfig 定义 Controller 通过 Envoy ExtProc 下发给 AI Proxy 的执行配置
package routeconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/lgc202/ingate/internal/pkg/httpheader"
	"github.com/lgc202/ingate/pkg/llm"
)

const (
	// GRPCMetadataKey 是模型 Route 向 ExtProc gRPC 流传递执行配置的 metadata key
	GRPCMetadataKey = "x-ingate-ai-route-config"
	// MaxRequestBodyBytes 限制文本模型请求的整体缓冲大小
	MaxRequestBodyBytes = 1 << 20
	// MaxResponseBodyBytes 限制需要整体转换的普通模型响应大小
	MaxResponseBodyBytes = 8 << 20
	// ResponseBufferLimitBytes 为 Envoy 连接缓冲预留一个 MiB 的越界检测空间
	ResponseBufferLimitBytes = MaxResponseBodyBytes + (1 << 20)
)

// Config 表示一条模型 RouteRule 的完整执行配置
type Config struct {
	RequireUsage bool       `json:"require_usage,omitempty"`
	Upstreams    []Upstream `json:"upstreams"`
	Models       []Model    `json:"models"`
}

// Upstream 表示模型路由可选择的一个实际模型服务
type Upstream struct {
	ID           string       `json:"id"`
	Protocol     llm.Protocol `json:"protocol"`
	Cluster      string       `json:"cluster"`
	Authority    string       `json:"authority"`
	BasePath     string       `json:"base_path"`
	APIKey       string       `json:"api_key,omitempty"`
	APIKeyHeader string       `json:"api_key_header,omitempty"`
	APIKeyPrefix string       `json:"api_key_prefix,omitempty"`
	Headers      []Header     `json:"headers,omitempty"`
}

// Header 表示发送给模型服务的固定请求 Header
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Model 表示客户端模型名称到实际模型服务和厂商模型名称的映射
type Model struct {
	Model         string `json:"model"`
	UpstreamID    string `json:"upstream_id"`
	UpstreamModel string `json:"upstream_model"`
}

// Encode 把执行配置编码成 ExtProc per-route gRPC metadata 使用的 JSON
func Encode(config Config) (string, error) {
	if err := config.validate(); err != nil {
		return "", err
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("encode AI route config: %w", err)
	}
	return string(raw), nil
}

// Decode 从 ExtProc gRPC metadata 严格解析执行配置
func Decode(encoded string) (Config, error) {
	if encoded == "" {
		return Config{}, errors.New("AI route config is missing")
	}
	var config Config
	if err := decodeStrict([]byte(encoded), &config); err != nil {
		return Config{}, err
	}
	if err := config.validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (c Config) validate() error {
	if len(c.Upstreams) == 0 {
		return errors.New("upstreams must not be empty")
	}
	if len(c.Models) == 0 {
		return errors.New("models must not be empty")
	}

	upstreams := make(map[string]bool, len(c.Upstreams))
	for i, upstream := range c.Upstreams {
		if err := upstream.validate(i); err != nil {
			return err
		}
		if upstreams[upstream.ID] {
			return fmt.Errorf("upstreams[%d] duplicates upstream %q", i, upstream.ID)
		}
		upstreams[upstream.ID] = true
	}

	models := make(map[string]bool, len(c.Models))
	for i, model := range c.Models {
		if model.Model == "" {
			return fmt.Errorf("models[%d].model must not be empty", i)
		}
		if model.UpstreamID == "" || !upstreams[model.UpstreamID] {
			return fmt.Errorf("models[%d] references missing upstream %q", i, model.UpstreamID)
		}
		if model.UpstreamModel == "" {
			return fmt.Errorf("models[%d].upstream_model must not be empty", i)
		}
		if models[model.Model] {
			return fmt.Errorf("models[%d] duplicates model %q", i, model.Model)
		}
		models[model.Model] = true
	}
	return nil
}

func (u Upstream) validate(index int) error {
	prefix := fmt.Sprintf("upstreams[%d]", index)
	if u.ID == "" {
		return fmt.Errorf("%s.id must not be empty", prefix)
	}
	switch u.Protocol {
	case llm.ProtocolOpenAIChatCompletions, llm.ProtocolAnthropicMessages, llm.ProtocolGeminiGenerateContent:
	default:
		return fmt.Errorf("%s.protocol %q is not supported", prefix, u.Protocol)
	}
	if u.Cluster == "" {
		return fmt.Errorf("%s.cluster must not be empty", prefix)
	}
	if u.Authority == "" || !httpheader.ValidValue(u.Authority) {
		return fmt.Errorf("%s.authority must be a valid header value", prefix)
	}
	if u.BasePath == "" || !strings.HasPrefix(u.BasePath, "/") {
		return fmt.Errorf("%s.base_path must start with /", prefix)
	}
	if u.APIKey == "" {
		if u.APIKeyHeader != "" || u.APIKeyPrefix != "" {
			return fmt.Errorf("%s API key header or prefix is set without an API key", prefix)
		}
	} else if u.APIKeyHeader == "" || !httpheader.ValidValue(u.APIKeyPrefix+u.APIKey) {
		return fmt.Errorf("%s API key configuration is invalid", prefix)
	}
	seenHeaders := make(map[string]bool, len(u.Headers))
	for i, header := range u.Headers {
		name := strings.ToLower(header.Name)
		if name == "" || header.Value == "" || !httpheader.ValidValue(header.Value) {
			return fmt.Errorf("%s.headers[%d] name and value must be valid", prefix, i)
		}
		if seenHeaders[name] {
			return fmt.Errorf("%s.headers[%d] duplicates header %q", prefix, i, header.Name)
		}
		seenHeaders[name] = true
	}
	return nil
}

func decodeStrict(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("decode AI route config: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("AI route config contains multiple JSON values")
		}
		return fmt.Errorf("decode AI route config: %w", err)
	}
	return nil
}

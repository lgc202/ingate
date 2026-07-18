package aiproxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/lgc202/ingate/internal/pkg/bearer"
)

// PluginConfig 表示真正下发给 AI Proxy Wasm 插件的运行时配置
type PluginConfig struct {
	Routes []RouteConfig `json:"routes"`
}

// RouteConfig 表示一条 RouteRule 的模型路由执行配置
type RouteConfig struct {
	GatewayName string        `json:"gatewayName"`
	RouteName   string        `json:"routeName"`
	RuleName    string        `json:"ruleName"`
	ConfigID    string        `json:"configID"`
	APIKey      string        `json:"apiKey,omitempty"`
	Models      []ModelConfig `json:"models"`
}

// ModelConfig 表示客户端模型名称到实际上游模型名称的可执行映射
type ModelConfig struct {
	Model         string `json:"model"`
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
		if len(route.Models) == 0 {
			return fmt.Errorf("routes[%d].models must not be empty", i)
		}
		if route.APIKey != "" && !bearer.ValidToken(route.APIKey) {
			return fmt.Errorf("routes[%d].apiKey contains unsupported whitespace or control characters", i)
		}

		routeKey := route.GatewayName + "\x00" + route.RouteName + "\x00" + route.RuleName
		if routes[routeKey] {
			return fmt.Errorf("routes[%d] duplicates route rule %q/%q/%q", i, route.GatewayName, route.RouteName, route.RuleName)
		}
		routes[routeKey] = true

		models := make(map[string]bool, len(route.Models))
		for j, model := range route.Models {
			if model.Model == "" {
				return fmt.Errorf("routes[%d].models[%d].model must not be empty", i, j)
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

package iprestriction

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// PluginConfig 表示真正下发给 IP 访问限制 Wasm 插件的 Listener 级执行配置
type PluginConfig struct {
	Routes []RouteConfig `json:"routes"`
}

// RouteConfig 表示 Route 上需要执行的全部 IP 访问限制策略
type RouteConfig struct {
	GatewayName string   `json:"gatewayName"`
	RouteName   string   `json:"routeName"`
	Policies    []Policy `json:"policies"`
}

// Policy 表示一条已经完成目标解析的 IP 允许列表或拒绝列表
type Policy struct {
	Name  string   `json:"name"`
	Allow []string `json:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty"`
}

// ParsePluginConfig 严格解析 Listener 级插件配置
func ParsePluginConfig(data []byte) (PluginConfig, error) {
	var config PluginConfig
	if err := decodeStrict(data, &config); err != nil {
		return PluginConfig{}, err
	}
	return config, nil
}

func decodeStrict(data []byte, value any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errors.New("IP restriction config must be a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("IP restriction config contains multiple JSON values")
		}
		return err
	}
	return nil
}

// Copyright 2026 Ingate Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Mock Response 使用标准 Proxy-Wasm ABI 在请求阶段直接返回固定 HTTP 响应
//
// 该实现借鉴 APISIX mocking 与 Higress custom-response 的本地响应语义，
// 但只保留 Ingate 强类型 MockResponsePolicy 当前开放的状态码、Header 和正文
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/net/http/httpguts"

	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

type pluginConfig struct {
	StatusCode uint32   `json:"statusCode"`
	Headers    []header `json:"headers"`
	Body       string   `json:"body"`
}

type header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type vmContext struct {
	types.DefaultVMContext
}

type pluginContext struct {
	types.DefaultPluginContext
	statusCode uint32
	headers    [][2]string
	body       []byte
}

type httpContext struct {
	types.DefaultHttpContext
	statusCode uint32
	headers    [][2]string
	body       []byte
}

func main() {}

func init() {
	proxywasm.SetVMContext(&vmContext{})
}

func (*vmContext) NewPluginContext(uint32) types.PluginContext {
	return &pluginContext{}
}

func (p *pluginContext) OnPluginStart(int) types.OnPluginStartStatus {
	rawConfig, err := proxywasm.GetPluginConfiguration()
	if err != nil {
		proxywasm.LogCriticalf("read plugin configuration: %v", err)
		return types.OnPluginStartStatusFailed
	}
	config, err := decodeConfig(rawConfig)
	if err != nil {
		proxywasm.LogCriticalf("invalid plugin configuration: %v", err)
		return types.OnPluginStartStatusFailed
	}

	p.statusCode = config.StatusCode
	p.headers = make([][2]string, 0, len(config.Headers))
	for _, header := range config.Headers {
		p.headers = append(p.headers, [2]string{header.Name, header.Value})
	}
	p.body = []byte(config.Body)
	return types.OnPluginStartStatusOK
}

// NewHttpContext 为每条 HTTP 流保存只读响应配置
//
//nolint:staticcheck // 方法名由 Proxy-Wasm SDK 接口定义，不能按 Go 缩写规则改名
func (p *pluginContext) NewHttpContext(uint32) types.HttpContext {
	return &httpContext{statusCode: p.statusCode, headers: p.headers, body: p.body}
}

// OnHttpRequestHeaders 在上游连接建立前结束请求，因此不会占用上游连接或产生重试
//
//nolint:staticcheck // 方法名由 Proxy-Wasm SDK 接口定义，不能按 Go 缩写规则改名
func (h *httpContext) OnHttpRequestHeaders(int, bool) types.Action {
	if err := proxywasm.SendHttpResponse(h.statusCode, h.headers, h.body, -1); err != nil {
		proxywasm.LogErrorf("send mock response: %v", err)
	}
	return types.ActionPause
}

func decodeConfig(raw []byte) (pluginConfig, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return pluginConfig{}, errors.New("configuration is empty")
	}
	var config pluginConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return pluginConfig{}, fmt.Errorf("decode JSON: %w", err)
	}
	if config.StatusCode < 200 || config.StatusCode > 599 {
		return pluginConfig{}, fmt.Errorf("status code %d is outside 200-599", config.StatusCode)
	}
	seen := make(map[string]struct{}, len(config.Headers))
	for index := range config.Headers {
		name := strings.ToLower(strings.TrimSpace(config.Headers[index].Name))
		if name == "" || strings.HasPrefix(name, ":") || !httpguts.ValidHeaderFieldName(name) {
			return pluginConfig{}, fmt.Errorf("header %d has invalid name %q", index+1, config.Headers[index].Name)
		}
		if _, exists := seen[name]; exists {
			return pluginConfig{}, fmt.Errorf("header %d duplicates %q", index+1, name)
		}
		if !httpguts.ValidHeaderFieldValue(config.Headers[index].Value) {
			return pluginConfig{}, fmt.Errorf("header %d contains invalid value bytes", index+1)
		}
		seen[name] = struct{}{}
		config.Headers[index].Name = name
	}
	return config, nil
}

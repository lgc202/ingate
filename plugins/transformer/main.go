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

// Package main 使用标准 Proxy-Wasm ABI 修改请求和响应 Header。
//
// 配置结构沿用 Higress Transformer 的 reqRules、respRules 与 Header 操作字段，
// 但只实现 Ingate 控制台当前开放的 remove、rename、replace、add 和 append。
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"

	"github.com/lgc202/ingate/internal/pkg/headertransformationconfig"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
)

type operation string

const (
	operationRemove  operation = "remove"
	operationRename  operation = "rename"
	operationReplace operation = "replace"
	operationAdd     operation = "add"
	operationAppend  operation = "append"
)

type pluginConfig struct {
	RequestRules  []rule `json:"reqRules"`
	ResponseRules []rule `json:"respRules"`
}

type rule struct {
	Operation operation    `json:"operate"`
	Headers   []headerRule `json:"headers"`
}

type headerRule struct {
	Key         string `json:"key,omitempty"`
	OldKey      string `json:"oldKey,omitempty"`
	NewKey      string `json:"newKey,omitempty"`
	Value       string `json:"value,omitempty"`
	NewValue    string `json:"newValue,omitempty"`
	AppendValue string `json:"appendValue,omitempty"`
}

type headerOperation struct {
	kind  operation
	key   string
	value string
}

type vmContext struct {
	types.DefaultVMContext
}

type pluginContext struct {
	types.DefaultPluginContext
	requestOperations  []headerOperation
	responseOperations []headerOperation
}

type httpContext struct {
	types.DefaultHttpContext
	requestOperations  []headerOperation
	responseOperations []headerOperation
}

// NewPluginContext 为每个 Root ID 创建独立插件上下文。
func (*vmContext) NewPluginContext(uint32) types.PluginContext {
	return &pluginContext{}
}

// OnPluginStart 读取并校验当前插件实例的 Header 转换配置。
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
	p.requestOperations, err = compileRules(config.RequestRules)
	if err != nil {
		proxywasm.LogCriticalf("invalid request rules: %v", err)
		return types.OnPluginStartStatusFailed
	}
	p.responseOperations, err = compileRules(config.ResponseRules)
	if err != nil {
		proxywasm.LogCriticalf("invalid response rules: %v", err)
		return types.OnPluginStartStatusFailed
	}
	if len(p.requestOperations)+len(p.responseOperations) == 0 {
		proxywasm.LogCritical("plugin configuration must contain at least one header rule")
		return types.OnPluginStartStatusFailed
	}
	return types.OnPluginStartStatusOK
}

// NewHttpContext 为每条 HTTP 流创建独立上下文。
//
//nolint:staticcheck // 方法名由 Proxy-Wasm SDK 接口定义，不能按 Go 缩写规则改名
func (p *pluginContext) NewHttpContext(uint32) types.HttpContext {
	return &httpContext{
		requestOperations:  p.requestOperations,
		responseOperations: p.responseOperations,
	}
}

// OnHttpRequestHeaders 在请求转发到上游前应用 Header 规则。
// Host ABI 失败会主动触发 VM trap，由 Envoy 的 FAIL_CLOSED 策略拒绝请求。
//
//nolint:staticcheck // 方法名由 Proxy-Wasm SDK 接口定义，不能按 Go 缩写规则改名
func (h *httpContext) OnHttpRequestHeaders(int, bool) types.Action {
	if len(h.requestOperations) == 0 {
		return types.ActionContinue
	}
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		proxywasm.LogCriticalf("read request headers: %v", err)
		panic("read request headers")
	}
	if err := proxywasm.ReplaceHttpRequestHeaders(applyOperations(headers, h.requestOperations)); err != nil {
		proxywasm.LogCriticalf("replace request headers: %v", err)
		panic("replace request headers")
	}
	return types.ActionContinue
}

// OnHttpResponseHeaders 在响应返回客户端前应用 Header 规则。
// Host ABI 失败会主动触发 VM trap，由 Envoy 的 FAIL_CLOSED 策略终止响应。
//
//nolint:staticcheck // 方法名由 Proxy-Wasm SDK 接口定义，不能按 Go 缩写规则改名
func (h *httpContext) OnHttpResponseHeaders(int, bool) types.Action {
	if len(h.responseOperations) == 0 {
		return types.ActionContinue
	}
	headers, err := proxywasm.GetHttpResponseHeaders()
	if err != nil {
		proxywasm.LogCriticalf("read response headers: %v", err)
		panic("read response headers")
	}
	if err := proxywasm.ReplaceHttpResponseHeaders(applyOperations(headers, h.responseOperations)); err != nil {
		proxywasm.LogCriticalf("replace response headers: %v", err)
		panic("replace response headers")
	}
	return types.ActionContinue
}

func main() {}

func init() {
	proxywasm.SetVMContext(&vmContext{})
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
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return pluginConfig{}, errors.New("configuration must contain exactly one JSON value")
	}
	return config, nil
}

func compileRules(rules []rule) ([]headerOperation, error) {
	operations := make([]headerOperation, 0)
	for ruleIndex, rule := range rules {
		if !rule.Operation.valid() {
			return nil, fmt.Errorf("rule %d has unsupported operation %q", ruleIndex+1, rule.Operation)
		}
		if len(rule.Headers) == 0 {
			return nil, fmt.Errorf("rule %d does not contain headers", ruleIndex+1)
		}
		for headerIndex, header := range rule.Headers {
			if len(operations) >= headertransformationconfig.MaxRules {
				return nil, fmt.Errorf("header operation count exceeds %d", headertransformationconfig.MaxRules)
			}
			compiled, err := rule.Operation.compile(header)
			if err != nil {
				return nil, fmt.Errorf("rule %d header %d: %w", ruleIndex+1, headerIndex+1, err)
			}
			operations = append(operations, compiled)
		}
	}
	return operations, nil
}

func (o operation) valid() bool {
	switch o {
	case operationRemove, operationRename, operationReplace, operationAdd, operationAppend:
		return true
	default:
		return false
	}
}

func (o operation) compile(rule headerRule) (headerOperation, error) {
	var key, value string
	switch o {
	case operationRename:
		key, value = httpheader.NormalizeName(rule.OldKey), httpheader.NormalizeName(rule.NewKey)
	case operationReplace:
		key, value = httpheader.NormalizeName(rule.Key), httpheader.NormalizeValue(rule.NewValue)
	case operationAdd:
		key, value = httpheader.NormalizeName(rule.Key), httpheader.NormalizeValue(rule.Value)
	case operationAppend:
		key, value = httpheader.NormalizeName(rule.Key), httpheader.NormalizeValue(rule.AppendValue)
	default:
		key = httpheader.NormalizeName(rule.Key)
	}
	if !httpheader.IsValidName(key) {
		return headerOperation{}, errors.New("header name is invalid")
	}
	if o == operationRename {
		if !httpheader.IsValidName(value) {
			return headerOperation{}, errors.New("new header name is invalid")
		}
	} else if o != operationRemove && !httpheader.IsValidValue(value) {
		return headerOperation{}, errors.New("header value is invalid")
	}
	return headerOperation{kind: o, key: key, value: value}, nil
}

// applyOperations 直接操作完整 Header 列表，以保留重复 Header 的顺序和值。
func applyOperations(headers [][2]string, operations []headerOperation) [][2]string {
	for _, operation := range operations {
		switch operation.kind {
		case operationRemove:
			headers = removeHeader(headers, operation.key)
		case operationRename:
			for index := range headers {
				if strings.EqualFold(headers[index][0], operation.key) {
					headers[index][0] = operation.value
				}
			}
		case operationReplace:
			headers = removeHeader(headers, operation.key)
			headers = append(headers, [2]string{operation.key, operation.value})
		case operationAdd:
			if !hasHeader(headers, operation.key) {
				headers = append(headers, [2]string{operation.key, operation.value})
			}
		case operationAppend:
			headers = append(headers, [2]string{operation.key, operation.value})
		}
	}
	return headers
}

func hasHeader(headers [][2]string, name string) bool {
	for _, header := range headers {
		if strings.EqualFold(header[0], name) {
			return true
		}
	}
	return false
}

func removeHeader(headers [][2]string, name string) [][2]string {
	result := headers[:0]
	for _, header := range headers {
		if !strings.EqualFold(header[0], name) {
			result = append(result, header)
		}
	}
	return result
}

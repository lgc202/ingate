// Package translator 按客户端与后端的协议组合适配 AI 请求。
// 当前只实现 OpenAI Chat Completions 的模型名覆盖，不负责路由、鉴权或 HTTP 传输。
package translator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"
)

// TranslateChatCompletionOpenAIToOpenAI 用已选后端的模型覆盖值生成独立的出站 JSON。
// body 须为经过 endpointspec.ParseChatCompletions 校验的同一份原始请求，调用期间不能修改。
// 覆盖值为空或与原模型相同时保留原始字节；其它覆盖只改变顶层 model 的值。
// 未改写字段以 RawMessage 保留，不转换数字或重建提供商类型，但排版与转义形式可能变化。
// 本函数不修改 body，不保存请求状态；每次返回的字节均与调用方的原始字节独立。
func TranslateChatCompletionOpenAIToOpenAI(body []byte, modelNameOverride string) ([]byte, error) {
	if modelNameOverride == "" {
		return bytes.Clone(body), nil
	}

	if !utf8.ValidString(modelNameOverride) {
		return nil, errors.New("model name override must be valid UTF-8")
	}

	// 每次从原始请求构建独立字段表，不能累积前一次适配的覆盖值。
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, fmt.Errorf("decode chat completion request for translation: %w", err)
	}

	var originalModel string
	if err := json.Unmarshal(fields["model"], &originalModel); err != nil {
		return nil, fmt.Errorf("decode original chat completion model: %w", err)
	}

	if originalModel == modelNameOverride {
		return bytes.Clone(body), nil
	}

	model, err := json.Marshal(modelNameOverride)
	if err != nil {
		return nil, fmt.Errorf("encode model name override: %w", err)
	}

	fields["model"] = model
	translated, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("encode chat completion request: %w", err)
	}

	return translated, nil
}

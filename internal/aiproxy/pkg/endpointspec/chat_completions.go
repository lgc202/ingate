// Package endpointspec 按客户端操作解析并校验 AI Proxy 请求。
// 本包不选择后端、不改写出站协议，也不负责 HTTP 读取或配置加载。
package endpointspec

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	sigsjson "sigs.k8s.io/json"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/apischema/openai"
)

// ParseChatCompletions 解析单个 UTF-8 JSON 对象，要求顶层 model 为非空字符串。
// model 区分大小写且不做修剪；拒绝重复顶层键，不校验完整提供商协议。
// 返回的协议字段独立于 body；调用方保留原始字节供后续适配，不用有限字段重新编码。
// 调用方须在读取请求体时限制大小；失败不返回部分结果，本函数不记录请求内容。
func ParseChatCompletions(body []byte) (*openai.ChatCompletionRequest, error) {
	if !utf8.Valid(body) {
		return nil, errors.New("chat completion request must be valid UTF-8")
	}

	var fields map[string]json.RawMessage
	strictErrors, err := sigsjson.UnmarshalStrict(body, &fields, sigsjson.DisallowDuplicateFields)
	if err != nil {
		return nil, fmt.Errorf("decode chat completion request: %w", err)
	}

	if len(strictErrors) != 0 {
		return nil, fmt.Errorf("decode chat completion request fields: %w", errors.Join(strictErrors...))
	}

	if fields == nil {
		return nil, errors.New("chat completion request must be a JSON object")
	}

	model, exists := fields["model"]
	if !exists {
		return nil, errors.New("chat completion request model is required")
	}

	var request openai.ChatCompletionRequest
	if err := json.Unmarshal(model, &request.Model); err != nil {
		return nil, fmt.Errorf("decode chat completion request model: %w", err)
	}

	if request.Model == "" {
		return nil, errors.New("chat completion request model must be a nonempty string")
	}

	return &request, nil
}

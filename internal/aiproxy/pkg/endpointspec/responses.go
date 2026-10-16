package endpointspec

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/apischema/openai"
)

// ParseResponses 校验创建操作的 JSON、原始模型名和非后台运行限制。
// 与 Chat Completions 共用 model 边界；不校验完整 input、工具或状态引用协议。
// 调用方限制正文大小并保留原始字节；background 为 true 时拒绝，不静默改写。
func ParseResponses(body []byte) (*openai.ResponseRequest, error) {
	fields, model, err := parseOpenAIRequest(body)
	if err != nil {
		return nil, err
	}

	request := openai.ResponseRequest{Model: model}
	if background, exists := fields["background"]; exists {
		if err := json.Unmarshal(background, &request.Background); err != nil {
			return nil, fmt.Errorf("decode response background: %w", err)
		}
	}

	if request.Background {
		return nil, errors.New("background responses are not supported")
	}

	return &request, nil
}

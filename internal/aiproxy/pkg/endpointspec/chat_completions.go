// Package endpointspec 按客户端操作解析并校验 AI Proxy 请求。
// 本包不选择后端、不改写出站协议，也不负责 HTTP 读取或配置加载。
package endpointspec

import "github.com/lgc202/ingate/internal/aiproxy/pkg/apischema/openai"

// ParseChatCompletions 解析单个 UTF-8 JSON 对象，要求顶层 model 为非空字符串。
// model 区分大小写且不做修剪；拒绝重复顶层键，不校验完整提供商协议。
// 返回的协议字段独立于 body；调用方保留原始字节供后续适配，不用有限字段重新编码。
// 调用方须在读取请求体时限制大小；失败不返回部分结果，本函数不记录请求内容。
func ParseChatCompletions(body []byte) (*openai.ChatCompletionRequest, error) {
	_, model, err := parseOpenAIRequest(body)
	if err != nil {
		return nil, err
	}

	return &openai.ChatCompletionRequest{Model: model}, nil
}

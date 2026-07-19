package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Role 表示文本消息在一次对话中的角色
type Role string

const (
	// RoleSystem 表示系统指令
	RoleSystem Role = "system"
	// RoleUser 表示用户输入
	RoleUser Role = "user"
	// RoleAssistant 表示模型回复
	RoleAssistant Role = "assistant"
)

// ChatRequest 表示第一阶段支持的 OpenAI-compatible Chat Completions 请求
//
// 指针字段用于区分调用方省略字段与显式传入零值，厂商转换时不会擅自补充采样参数
type ChatRequest struct {
	Model       string
	Messages    []Message
	Stream      *bool
	Temperature *float64
	TopP        *float64
	MaxTokens   *int
	Stop        []string

	unknownFields []string
}

// Message 表示第一阶段支持的纯文本消息
type Message struct {
	Role    Role
	Content string

	unknownFields []string
}

// Streaming 返回请求是否启用 SSE 流式响应
func (r ChatRequest) Streaming() bool {
	return r.Stream != nil && *r.Stream
}

// ValidateSupported 确认请求只使用第一阶段公开支持的字段
func (r ChatRequest) ValidateSupported() error {
	if len(r.unknownFields) > 0 {
		return fmt.Errorf("%w: fields %s are not supported",
			ErrUnsupportedFeature, strings.Join(r.unknownFields, ", "))
	}

	seenConversationMessage := false
	for i, message := range r.Messages {
		if len(message.unknownFields) > 0 {
			return fmt.Errorf("%w: messages[%d] fields %s are not supported",
				ErrUnsupportedFeature, i, strings.Join(message.unknownFields, ", "))
		}
		if message.Role == RoleSystem {
			if seenConversationMessage {
				return fmt.Errorf("%w: messages[%d] system message must precede user and assistant messages",
					ErrUnsupportedFeature, i)
			}
			continue
		}
		seenConversationMessage = true
	}
	return nil
}

// DecodeChatRequest 解析并校验第一阶段支持的 OpenAI-compatible 请求
//
// 未知字段会被记录，由 ValidateSupported 统一拒绝，避免不同模型厂商暴露不同的外部协议
func DecodeChatRequest(body []byte) (ChatRequest, error) {
	if !isJSONObject(body) {
		return ChatRequest{}, fmt.Errorf("%w: request body must be a JSON object", ErrInvalidRequest)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return ChatRequest{}, fmt.Errorf("%w: decode request body: %v", ErrInvalidRequest, err)
	}

	for _, name := range []string{"tools", "tool_choice", "functions", "function_call", "parallel_tool_calls"} {
		if value, ok := fields[name]; ok && !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return ChatRequest{}, fmt.Errorf("%w: field %q is not supported", ErrUnsupportedFeature, name)
		}
	}

	request := ChatRequest{}
	if err := decodeRequiredString(fields, "model", &request.Model); err != nil {
		return ChatRequest{}, err
	}
	if strings.TrimSpace(request.Model) == "" {
		return ChatRequest{}, fmt.Errorf("%w: field %q must not be empty", ErrInvalidRequest, "model")
	}

	messages, err := decodeMessages(fields["messages"])
	if err != nil {
		return ChatRequest{}, err
	}
	request.Messages = messages

	if err := decodeOptional(fields, "stream", &request.Stream); err != nil {
		return ChatRequest{}, err
	}
	if err := decodeOptional(fields, "temperature", &request.Temperature); err != nil {
		return ChatRequest{}, err
	}
	if request.Temperature != nil && (*request.Temperature < 0 || *request.Temperature > 2) {
		return ChatRequest{}, fmt.Errorf("%w: field %q must be between 0 and 2", ErrInvalidRequest, "temperature")
	}
	if err := decodeOptional(fields, "top_p", &request.TopP); err != nil {
		return ChatRequest{}, err
	}
	if request.TopP != nil && (*request.TopP < 0 || *request.TopP > 1) {
		return ChatRequest{}, fmt.Errorf("%w: field %q must be between 0 and 1", ErrInvalidRequest, "top_p")
	}
	if err := decodeOptional(fields, "max_tokens", &request.MaxTokens); err != nil {
		return ChatRequest{}, err
	}
	if request.MaxTokens != nil && *request.MaxTokens <= 0 {
		return ChatRequest{}, fmt.Errorf("%w: field %q must be greater than 0", ErrInvalidRequest, "max_tokens")
	}
	if raw, ok := fields["stop"]; ok {
		request.Stop, err = decodeStop(raw)
		if err != nil {
			return ChatRequest{}, err
		}
	}

	supported := []string{"model", "messages", "stream", "temperature", "top_p", "max_tokens", "stop"}
	for name := range fields {
		if !slices.Contains(supported, name) {
			request.unknownFields = append(request.unknownFields, name)
		}
	}
	sort.Strings(request.unknownFields)

	return request, nil
}

func decodeMessages(raw json.RawMessage) ([]Message, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: field %q is required", ErrInvalidRequest, "messages")
	}

	var values []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("%w: field %q must be an array of message objects", ErrInvalidRequest, "messages")
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("%w: field %q must not be empty", ErrInvalidRequest, "messages")
	}

	messages := make([]Message, 0, len(values))
	for i, fields := range values {
		if fields == nil {
			return nil, fmt.Errorf("%w: messages[%d] must be an object", ErrInvalidRequest, i)
		}

		for _, name := range []string{"tool_calls", "function_call"} {
			if value, ok := fields[name]; ok && !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return nil, fmt.Errorf("%w: messages[%d].%s is not supported", ErrUnsupportedFeature, i, name)
			}
		}

		var role Role
		if err := decodeRequiredString(fields, "role", &role); err != nil {
			return nil, fmt.Errorf("%w: messages[%d].role must be system, user, or assistant", ErrInvalidRequest, i)
		}
		if !validRole(role) {
			return nil, fmt.Errorf("%w: messages[%d].role %q is not supported", ErrUnsupportedFeature, i, role)
		}

		rawContent, ok := fields["content"]
		if !ok {
			return nil, fmt.Errorf("%w: messages[%d].content is required", ErrInvalidRequest, i)
		}
		trimmedContent := bytes.TrimSpace(rawContent)
		if len(trimmedContent) > 0 && (trimmedContent[0] == '[' || trimmedContent[0] == '{') {
			return nil, fmt.Errorf("%w: messages[%d].content must be plain text", ErrUnsupportedFeature, i)
		}
		var content string
		if err := decodeRequiredString(fields, "content", &content); err != nil {
			return nil, fmt.Errorf("%w: messages[%d].content must be a string", ErrInvalidRequest, i)
		}

		message := Message{Role: role, Content: content}
		for name := range fields {
			if name != "role" && name != "content" {
				message.unknownFields = append(message.unknownFields, name)
			}
		}
		sort.Strings(message.unknownFields)
		messages = append(messages, message)
	}
	return messages, nil
}

func decodeRequiredString[T ~string](fields map[string]json.RawMessage, name string, target *T) error {
	raw, ok := fields[name]
	if !ok {
		return fmt.Errorf("%w: field %q is required", ErrInvalidRequest, name)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("%w: field %q must be a string", ErrInvalidRequest, name)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("%w: field %q must be a string", ErrInvalidRequest, name)
	}
	return nil
}

func decodeOptional[T any](fields map[string]json.RawMessage, name string, target **T) error {
	raw, ok := fields[name]
	if !ok {
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("%w: field %q has an invalid type", ErrInvalidRequest, name)
	}
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("%w: field %q has an invalid type", ErrInvalidRequest, name)
	}
	*target = &value
	return nil
}

func decodeStop(raw json.RawMessage) ([]string, error) {
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, nil
	}

	var multiple []string
	if err := json.Unmarshal(raw, &multiple); err != nil || multiple == nil {
		return nil, fmt.Errorf("%w: field %q must be a string or an array of strings", ErrInvalidRequest, "stop")
	}
	return multiple, nil
}

func isJSONObject(data []byte) bool {
	data = bytes.TrimSpace(data)
	return len(data) >= 2 && data[0] == '{' && data[len(data)-1] == '}'
}

func validRole(role Role) bool {
	switch role {
	case RoleSystem, RoleUser, RoleAssistant:
		return true
	default:
		return false
	}
}

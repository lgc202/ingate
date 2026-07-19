package llm

import "errors"

var (
	// ErrInvalidRequest 表示客户端请求不符合第一阶段文本 Chat 协议
	ErrInvalidRequest = errors.New("invalid chat completion request")
	// ErrUnsupportedFeature 表示请求或响应使用了第一阶段未支持的能力
	ErrUnsupportedFeature = errors.New("unsupported chat completion feature")
	// ErrInvalidResponse 表示上游普通响应无法转换
	ErrInvalidResponse = errors.New("invalid upstream chat completion response")
	// ErrInvalidStream 表示上游 SSE 事件或状态序列无法转换
	ErrInvalidStream = errors.New("invalid upstream chat completion stream")
)

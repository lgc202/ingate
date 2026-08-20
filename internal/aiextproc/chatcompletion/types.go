// Package chatcompletion 转换 OpenAI Chat Completions 与模型服务协议
package chatcompletion

// UpstreamRequest 是送往模型服务的请求体及其变化状态
type UpstreamRequest struct {
	Body        []byte
	BodyChanged bool // 首次上游尝试只有在正文变化时才需要覆盖 Envoy 当前正文
}

// RequestMetadata 是入口阶段从 OpenAI Chat Completions 请求中提取的路由信息
type RequestMetadata struct {
	Model     string
	Streaming bool
}

// Usage 是不同模型服务协议归一后的 Token 用量
type Usage struct {
	InputTokens  uint64
	OutputTokens uint64
	TotalTokens  uint64
	Found        bool // 区分明确返回零 Token 与响应中未提供用量
}

// ResponseMetadata 是从模型服务响应中提取的运行信息
// ExtProc 把这些字段写入动态元数据，ALS 无需再次解析响应正文
type ResponseMetadata struct {
	ResponseModel string
	FinishReason  string
	Usage         Usage
}

// InvalidRequestError 表示能够安全返回给调用方的请求协议错误
type InvalidRequestError struct {
	message string
}

func (e *InvalidRequestError) Error() string {
	return e.message
}

// Message 返回不包含内部实现信息的客户端错误文案
func (e *InvalidRequestError) Message() string {
	return e.message
}

func invalidRequest(message string) error {
	return &InvalidRequestError{message: message}
}

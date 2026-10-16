package openai

// ErrorResponse 表达响应开始前本地产生的 OpenAI HTTP 错误，不是流式错误事件。
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail 保存安全的公开错误；未确定的参数和错误码编码为 null。
// Code 可以使用网关自己的公开原因码，不表示该原因由 OpenAI 服务产生。
type ErrorDetail struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    *string `json:"code"`
}

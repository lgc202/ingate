package openai

// ResponseRequest 表达 Responses 创建操作当前需要读取的字段，不代表完整提供商请求。
// 原始 input、工具、状态引用及扩展字段由调用方保留，不用本类型重新编码正文。
type ResponseRequest struct {
	Model      string `json:"model"`
	Background bool   `json:"background"`
}

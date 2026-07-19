// Package policy 执行 OpenAI-compatible 请求的模型选择
package policy

import config "github.com/lgc202/ingate/pkg/plugin/aiproxy"

// Request 表示 AI Proxy 判断需要读取的请求信息
type Request struct {
	Method string
	Path   string
	Body   []byte
}

// Selection 表示根据公开模型名称选中的执行目标
type Selection struct {
	Model  config.ModelConfig
	Stream bool
}

// Rejection 表示 OpenAI-compatible 本地错误响应
type Rejection struct {
	StatusCode int
	Message    string
	Type       string
	Param      string
	Code       string
	Allow      string
}

// Decision 表示 AI Proxy 对一次请求的模型选择结果
type Decision struct {
	Selection *Selection
	Rejection *Rejection
}

// Runner 根据当前 RouteRule 的模型索引处理 OpenAI-compatible 请求
type Runner struct{}

// NewRunner 创建 AI Proxy 请求执行器
func NewRunner() *Runner {
	return &Runner{}
}

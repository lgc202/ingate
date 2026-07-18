// Package policy 执行 OpenAI-compatible 请求的模型选择和请求体改写
package policy

// Request 表示 AI Proxy 判断需要读取的请求信息
type Request struct {
	Method string
	Path   string
	Body   []byte
}

// Mutation 表示发送给模型 Upstream 前需要应用的请求变更
type Mutation struct {
	Body []byte
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

// Decision 表示 AI Proxy 对一次请求的处理结果
type Decision struct {
	Mutation  Mutation
	Rejection *Rejection
}

// Runner 根据当前 RouteRule 的模型索引处理 OpenAI-compatible 请求
type Runner struct{}

// NewRunner 创建 AI Proxy 请求执行器
func NewRunner() *Runner {
	return &Runner{}
}

package modelconfig

import "time"

// Mode 表示运维助手请求模型的网络路径。
type Mode uint8

const (
	ModeDirect Mode = iota + 1
	ModeIngate
)

// Protocol 表示 Assistant 与模型端点之间使用的请求协议。
type Protocol uint8

const (
	ProtocolOpenAICompatible Protocol = iota + 1
	ProtocolAnthropic
)

const (
	DefaultTimeout         = 120 * time.Second
	DefaultMaxOutputTokens = 4096
	minReasoningBudget     = 1024
	maxTimeout             = 30 * time.Minute
	maxOutputTokens        = 1_000_000
	maxEndpointLength      = 2048
	maxModelLength         = 160
	maxAPIKeyLength        = 4096
)

// Connection 描述运维助手访问模型所需的当前连接。
// Configured 仅用于区分首次配置表单和已经持久化的连接。
type Connection struct {
	Configured      bool
	Mode            Mode
	Protocol        Protocol
	Endpoint        string
	APIKey          string
	Model           string
	Timeout         time.Duration
	MaxOutputTokens int
	// ReasoningBudgetTokens 大于 0 时为 Anthropic 请求开启扩展思考。
	// OpenAI 兼容模型是否返回推理内容由对应端点决定。
	ReasoningBudgetTokens int
	UpdatedAt             time.Time
}

// Update 使用可空 APIKey 区分“保留原凭据”和“写入新凭据”。
// 显式传入空字符串表示清空凭据。
type Update struct {
	Connection Connection
	APIKey     *string
}

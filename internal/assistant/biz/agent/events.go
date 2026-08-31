// Package agent 实现 Ingate 运维 Agent 的模型循环、工具调度和执行事件。
package agent

import (
	"context"
	"errors"
)

const (
	// RoleUser 表示来自管理员的上下文消息。
	RoleUser Role = iota + 1
	// RoleAssistant 表示此前由 Agent 生成的上下文消息。
	RoleAssistant
)

var (
	// ErrModelNotConfigured 表示当前没有可用于运维助手的模型连接。
	ErrModelNotConfigured = errors.New("assistant model is not configured")
	// ErrToolUnavailable 表示工具所依赖的 Ingate 内部服务当前不可用。
	ErrToolUnavailable = errors.New("assistant tool is unavailable")
	// ErrIterationLimit 表示模型未能在限定轮数内结束工具调用循环。
	ErrIterationLimit = errors.New("assistant exceeded the model iteration limit")
)

// Role 表示进入模型上下文的消息角色。
// 工具消息由 Eino 在单次循环内维护，不属于跨执行恢复的持久对话。
type Role uint8

// Message 是 Agent 恢复上下文时需要的最小消息结构。
// ID、会话归属和创建时间属于存储事实，不应进入模型循环协议。
type Message struct {
	Role    Role
	Content string
}

// Request 是一次 Agent 执行的不可变请求。
// 历史消息由执行编排层一次性读取，Agent 循环只在内存中追加模型和工具消息。
type Request struct {
	Messages []Message
}

// Response 是 Agent 自然结束后产生的用户可见响应。
// 工具参数和原始工具结果属于循环上下文，不会通过该对象进入持久消息。
type Response struct {
	Content          string
	ReasoningContent string
}

// Event 是 Agent 循环对外发布的封闭事件集合。
//
// 具体事件使用独立结构体，而不是复用 Name、Data 等弱类型字段。这样新增事件时，
// 生产者和消费者都能从类型本身看出有效字段，也不会把模型名误写成工具名。
type Event interface {
	agentEvent()
}

// ModelSelected 表示本次执行已经确定要使用的模型。
// 它先于远端模型客户端创建，便于连接失败时仍保留明确的选模事实。
type ModelSelected struct {
	Model string
}

// ModelCallStarted 表示一次真实模型请求已经开始。
type ModelCallStarted struct {
	CallID string
	Model  string
}

// ModelCallCompleted 表示一次模型请求正常返回。
type ModelCallCompleted struct {
	CallID string
	Model  string
	// Summary 只能包含允许在执行详情中展示的脱敏摘要。
	Summary string
}

// ToolCallStarted 表示模型选择的工具已经开始执行。
type ToolCallStarted struct {
	CallID string
	Tool   string
}

// ToolCallCompleted 表示工具正常返回或返回了可由模型修正的参数错误。
type ToolCallCompleted struct {
	CallID string
	Tool   string
	// Summary 是面向用户的执行摘要，不保存工具参数或原始结果。
	Summary string
}

// ToolCallFailed 表示工具依赖不可用或执行结果无法解释。
type ToolCallFailed struct {
	CallID string
	Tool   string
}

// ReasoningDelta 是厂商显式返回的一段思考内容。
type ReasoningDelta struct {
	Content string
}

// ContentDelta 是最终回答的一段增量内容。
type ContentDelta struct {
	Content string
}

// EventSink 接收 Agent 执行过程中产生的事件。
//
// Emit 是同步边界：调用方可以让执行步骤的持久化错误立即中止模型循环，也可以把
// 短期流式通知降级为尽力写入。这里有意不使用异步 Channel，调用方天然形成背压。
type EventSink interface {
	Emit(context.Context, Event) error
}

func (ModelSelected) agentEvent() {}

func (ModelCallStarted) agentEvent() {}

func (ModelCallCompleted) agentEvent() {}

func (ToolCallStarted) agentEvent() {}

func (ToolCallCompleted) agentEvent() {}

func (ToolCallFailed) agentEvent() {}

func (ReasoningDelta) agentEvent() {}

func (ContentDelta) agentEvent() {}

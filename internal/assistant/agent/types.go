// Package agent 定义运维 Agent 的稳定执行协议。
//
// 这里的类型只描述一次 Agent 执行需要的输入、过程事件和最终结果，
// 不携带 MySQL 租约、Redis 事件或具体模型 SDK 对象。
package agent

import (
	"context"
	"errors"
)

var (
	// ErrModelNotConfigured 表示当前没有可用于运维助手的模型连接。
	ErrModelNotConfigured = errors.New("assistant model is not configured")
	// ErrToolUnavailable 表示工具所依赖的 Ingate 内部服务当前不可用。
	ErrToolUnavailable = errors.New("assistant tool is unavailable")
)

// Role 表示进入模型上下文的消息角色。
// 工具消息由 Eino 在单次循环内维护，不属于跨执行恢复的持久对话。
type Role uint8

const (
	RoleUser Role = iota + 1
	RoleAssistant
)

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

func (ModelSelected) agentEvent() {}

// ModelCallStarted 表示一次真实模型请求已经开始。
type ModelCallStarted struct {
	CallID string
	Model  string
}

func (ModelCallStarted) agentEvent() {}

// ModelCallCompleted 表示一次模型请求正常返回。
type ModelCallCompleted struct {
	CallID string
	Model  string
	// Summary 只能包含允许在执行详情中展示的脱敏摘要。
	Summary string
}

func (ModelCallCompleted) agentEvent() {}

// ToolCallStarted 表示模型选择的工具已经开始执行。
type ToolCallStarted struct {
	CallID string
	Tool   string
}

func (ToolCallStarted) agentEvent() {}

// ToolCallCompleted 表示工具正常返回或返回了可由模型修正的参数错误。
type ToolCallCompleted struct {
	CallID string
	Tool   string
	// Summary 是面向用户的执行摘要，不保存工具参数或原始结果。
	Summary string
}

func (ToolCallCompleted) agentEvent() {}

// ToolCallFailed 表示工具依赖不可用或执行结果无法解释。
type ToolCallFailed struct {
	CallID string
	Tool   string
}

func (ToolCallFailed) agentEvent() {}

// ReasoningDelta 是厂商显式返回的一段思考内容。
type ReasoningDelta struct {
	Content string
}

func (ReasoningDelta) agentEvent() {}

// ContentDelta 是最终回答的一段增量内容。
type ContentDelta struct {
	Content string
}

func (ContentDelta) agentEvent() {}

// EventSink 接收 Agent 执行过程中产生的事件。
//
// Emit 是同步边界：事件无法持久化时立即中止当前执行，避免模型和工具继续产生
// 无法归属到本次执行的结果。这里有意不使用异步 Channel，调用方天然形成背压。
type EventSink interface {
	Emit(context.Context, Event) error
}

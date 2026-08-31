package agent

import (
	"context"

	changebiz "github.com/lgc202/ingate/internal/assistant/biz/change"
)

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

// ChangeCompleted 表示获批的配置工具已经得到可收敛的写入结果。
type ChangeCompleted struct {
	ID         string
	State      changebiz.State
	ResourceID string
	ErrorCode  changebiz.FailureCode
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

func (ChangeCompleted) agentEvent() {}

func (ToolCallFailed) agentEvent() {}

func (ReasoningDelta) agentEvent() {}

func (ContentDelta) agentEvent() {}

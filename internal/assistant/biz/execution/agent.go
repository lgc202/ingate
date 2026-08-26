package execution

import (
	"context"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
)

// Agent 表示后台执行器需要的推理能力。
// 接口定义在调用方，业务编排不依赖 Eino、模型 SDK 或具体工具实现。
type Agent interface {
	Execute(context.Context, AgentRequest) (AgentResult, error)
}

// DeltaType 区分模型流中的推理内容和最终回答。
type DeltaType uint8

const (
	DeltaReasoning DeltaType = iota + 1
	DeltaContent
)

// Delta 是模型返回的一段增量内容。
type Delta struct {
	Type    DeltaType
	Content string
}

// AgentRequest 汇集一次推理所需的上下文和生命周期回调。
// Agent 只负责推理，执行状态、租约和事件持久化仍由 Executor 负责。
type AgentRequest struct {
	Messages    []conversation.Message
	Recorder    StepRecorder
	SelectModel func(context.Context, string) error
	Emit        func(Delta) error
}

// AgentResult 是 Agent 最终需要持久化的用户可见结果。
type AgentResult struct {
	Content          string
	ReasoningContent string
}

// StepRecorder 记录 Agent 循环中真正发生的模型与工具调用。
// 参数和原始工具结果不得经过这个边界，避免执行追踪成为敏感数据副本。
type StepRecorder interface {
	ModelStarted(context.Context, string, string) error
	ModelCompleted(context.Context, string, string) error
	ToolStarted(context.Context, string, string) error
	ToolCompleted(context.Context, string, string) error
	ToolFailed(context.Context, string) error
}

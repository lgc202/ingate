package run

import (
	"context"
	"time"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
)

// Store 由 Run 业务定义持久化边界，MySQL 实现领取、租约和终态事务。
type Store interface {
	ListRecentMessages(context.Context, string, string, int) ([]conversation.Message, error)
	CreateRun(context.Context, string, string, string) (Run, error)
	ClaimRun(context.Context, string, time.Duration) (Claimed, bool, error)
	SetRunModel(context.Context, string, string, string) error
	StartRunItem(context.Context, string, string, Item) (Item, error)
	CompleteRunItem(context.Context, string, string, string, ItemKind, string) error
	FailRunItem(context.Context, string, string, string, ItemKind, FailureCode) error
	ListRunItems(context.Context, string, string) ([]Item, error)
	RenewRunLease(context.Context, string, string, time.Duration) (bool, error)
	CompleteRun(context.Context, string, string, string, Result) (conversation.Message, error)
	FailRun(context.Context, string, string, string, FailureCode) error
	CancelRun(context.Context, string, string) (Run, error)
	FinishRunCancellation(context.Context, string, string) error
	FailExpiredRuns(context.Context) (int64, error)
	GetRun(context.Context, string, string) (Run, error)
}

// EventStore 保存可过期的 Run 事件，不能作为消息或 Run 状态的事实来源。
type EventStore interface {
	Append(context.Context, string, StreamEvent) (string, error)
	Read(context.Context, string, string, int64, time.Duration) ([]StreamEvent, error)
}

// ExecutionRecorder 记录 Eino 执行循环中已经实际发生的模型和工具调用。
// 业务层只接收稳定名称和脱敏摘要，不接触工具参数或原始结果。
type ExecutionRecorder interface {
	ModelStarted(context.Context, string, string) error
	ModelCompleted(context.Context, string, string) error
	ToolStarted(context.Context, string, string) error
	ToolCompleted(context.Context, string, string) error
	ToolFailed(context.Context, string) error
}

// AgentRequest 是一次 Agent 执行需要的历史消息和业务回调。
// 模型选择和增量事件必须先经过 Run 业务层，数据适配层不能直接修改持久状态。
type AgentRequest struct {
	Messages    []conversation.Message
	Recorder    ExecutionRecorder
	SelectModel func(context.Context, string) error
	Emit        func(Delta) error
}

// Agent 隔离 Eino 的消息和流式类型，业务层只依赖项目领域对象。
type Agent interface {
	Execute(context.Context, AgentRequest) (Result, error)
}

package execution

import (
	"context"
	"time"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
)

// Store 由执行业务定义持久化边界，MySQL 实现领取、租约和终态事务。
type Store interface {
	ListRecentMessages(context.Context, string, string, int) ([]conversation.Message, error)
	CreateExecution(context.Context, string, string, string) (AgentExecution, error)
	GetExecution(context.Context, string, string) (AgentExecution, error)
	ListExecutionSteps(context.Context, string, string) ([]Step, error)
	CancelExecution(context.Context, string, string) (AgentExecution, error)

	ClaimExecution(context.Context, string, time.Duration) (ClaimedExecution, bool, error)
	SetExecutionModel(context.Context, string, string, string) error
	StartExecutionStep(context.Context, string, string, Step) (Step, error)
	CompleteExecutionStep(context.Context, string, string, string, StepKind, string) error
	FailExecutionStep(context.Context, string, string, string, StepKind, FailureCode) error
	RenewExecutionLease(context.Context, string, string, time.Duration) (bool, error)
	CompleteExecution(context.Context, string, string, string, AgentResult) (conversation.Message, error)
	FailExecution(context.Context, string, string, string, FailureCode) error
	FinishExecutionCancellation(context.Context, string, string) error
	FailExpiredExecutions(context.Context) (int64, error)
}

// EventStore 保存可过期的执行事件，不能作为消息或执行状态的事实来源。
type EventStore interface {
	Append(context.Context, string, StreamEvent) (string, error)
	Read(context.Context, string, string, int64, time.Duration) ([]StreamEvent, error)
}

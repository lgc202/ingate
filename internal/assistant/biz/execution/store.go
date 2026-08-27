package execution

import (
	"context"
	"time"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
)

// ServiceStore 是面向 HTTP API 的持久化边界。
// 它只暴露用户能够发起的操作，不把后台任务的领取、租约和终态提交能力带入请求链路。
type ServiceStore interface {
	CreateExecution(context.Context, string, string, string) (Execution, error)
	GetExecution(context.Context, string, string) (Execution, error)
	ListExecutionSteps(context.Context, string, string) ([]Step, error)
	CancelExecution(context.Context, string, string) (Execution, error)
}

// ExecutorStore 是后台执行器使用的持久化边界。
// 领取、续租、步骤和终态必须由同一个存储实现协调，才能保证一次执行只由租约持有者提交。
type ExecutorStore interface {
	ListRecentMessages(context.Context, string, string, int) ([]conversation.Message, error)
	ClaimExecution(context.Context, string, time.Duration) (Claim, bool, error)
	SetExecutionModel(context.Context, string, string, string) error
	StartExecutionStep(context.Context, string, string, Step) (Step, error)
	CompleteExecutionStep(context.Context, string, string, string, StepKind, string) error
	FailExecutionStep(context.Context, string, string, string, StepKind, FailureCode) error
	RenewExecutionLease(context.Context, string, string, time.Duration) (bool, error)
	CompleteExecution(context.Context, string, string, string, Completion) (conversation.Message, error)
	FailExecution(context.Context, string, string, string, FailureCode) error
	FinishExecutionCancellation(context.Context, string, string) error
	FailExpiredExecutions(context.Context) (int64, error)
}

// EventStore 保存可过期的执行事件，不能作为消息或执行状态的事实来源。
type EventStore interface {
	Append(context.Context, string, StreamEvent) (string, error)
	Read(context.Context, string, string, int64, time.Duration) ([]StreamEvent, error)
}

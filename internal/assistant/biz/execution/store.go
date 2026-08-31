package execution

import (
	"context"
	"time"

	agentbiz "github.com/lgc202/ingate/internal/assistant/biz/agent"
	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
)

// Store 是执行 Usecase 需要的持久化边界。
// 它只暴露用户能够发起的操作，
// 不把后台任务的领取、租约和终态提交能力带入请求链路。
type Store interface {
	CreateExecution(context.Context, string, string, string) (Execution, error)
	GetExecution(context.Context, string, string) (Execution, error)
	ListExecutionSteps(context.Context, string, string) ([]Step, error)
	CancelExecution(context.Context, string, string) (Execution, error)
}

// ExecutorStore 是后台执行器使用的持久化边界。
// 领取、续租、步骤和终态必须由同一个存储实现协调，
// 才能保证一次执行只由租约持有者提交。
type ExecutorStore interface {
	ListRecentMessages(context.Context, string, string, int, int64) ([]conversation.HistoryMessage, error)
	ClaimExecution(context.Context, string, time.Duration) (Claim, bool, error)
	BindExecutionModel(context.Context, string, string, string) error
	StartExecutionStep(context.Context, string, string, Step) error
	CompleteExecutionStep(context.Context, string, string, string, StepKind, string) error
	FailExecutionStep(context.Context, string, string, string, StepKind, FailureCode) error
	RenewExecutionLease(context.Context, string, string, time.Duration) (bool, error)
	CompleteExecution(context.Context, string, string, string, Completion) (conversation.Message, error)
	PauseExecution(context.Context, string, string, string, agentbiz.ApprovalInterruption) error
	FailExecution(context.Context, string, string, string, FailureCode) error
	FinishExecutionCancellation(context.Context, string, string, string) error
	FailExpiredExecutions(context.Context) (int64, error)
}

// EventStore 保存可过期的执行事件，不能作为消息或执行状态的事实来源。
type EventStore interface {
	Append(context.Context, string, StreamEvent) (string, error)
	Read(context.Context, string, string, int64, time.Duration) ([]StreamEvent, error)
}

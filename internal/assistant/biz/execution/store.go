package execution

import (
	"context"
	"time"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
)

// StepStart 包含开始一次模型或工具调用所需的数据，序号、状态和时间由持久存储确定。
type StepStart struct {
	ID     string
	Kind   StepKind
	Name   string
	CallID string
}

// Store 是执行 Usecase 需要的持久化边界。
// 它只暴露用户能够发起的操作，
// 不把后台任务的领取、租约和终态提交能力带入请求链路。
type Store interface {
	CreateExecution(ctx context.Context, actorID, conversationID, content string) (Execution, error)
	GetExecution(ctx context.Context, actorID, id string) (Execution, error)
	ListExecutionSteps(ctx context.Context, actorID, executionID string) ([]Step, error)
	CancelExecution(ctx context.Context, actorID, executionID string) (Execution, error)
}

// ExecutorStore 是后台执行器使用的持久化边界。
// 领取、续租、步骤和终态必须由同一个存储实现协调，
// 才能保证一次执行只由租约持有者提交。
type ExecutorStore interface {
	ListRecentMessages(
		ctx context.Context,
		actorID string,
		conversationID string,
		maxMessages int,
		maxContentBytes int64,
	) ([]conversation.HistoryMessage, error)
	ClaimExecution(ctx context.Context, workerID string, leaseDuration time.Duration) (Claim, bool, error)
	SetExecutionModel(ctx context.Context, executionID, workerID, model string) error
	StartExecutionStep(ctx context.Context, executionID, workerID string, step StepStart) error
	CompleteExecutionStep(ctx context.Context, executionID, workerID, callID string, kind StepKind, summary string) error
	FailExecutionStep(ctx context.Context, executionID, workerID, callID string, kind StepKind, code FailureCode) error
	RenewExecutionLease(ctx context.Context, executionID, workerID string, leaseDuration time.Duration) (bool, error)
	CompleteExecution(
		ctx context.Context,
		actorID string,
		executionID string,
		workerID string,
		result Completion,
	) (conversation.Message, error)
	FailExecution(ctx context.Context, actorID, executionID, workerID string, errorCode FailureCode) error
	FinishExecutionCancellation(ctx context.Context, actorID, executionID, workerID string) error
	FailExpiredExecutions(ctx context.Context) (int64, error)
}

// EventStore 保存可过期的执行事件，不能作为消息或执行状态的事实来源。
type EventStore interface {
	Append(ctx context.Context, executionID string, event StreamEvent) (string, error)
	Read(ctx context.Context, executionID, lastID string, limit int64, block time.Duration) ([]StreamEvent, error)
}

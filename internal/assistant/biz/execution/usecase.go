package execution

import (
	"context"
	"time"
)

// Usecase 提供执行对象的创建、查询、取消和事件读取能力。
// 模型调用由 Executor 承担，API 请求不会同步执行耗时任务。
type Usecase struct {
	store  Store
	events EventStore
}

// NewUsecase 创建面向 API 的执行业务入口。
func NewUsecase(store Store, events EventStore) *Usecase {
	return &Usecase{store: store, events: events}
}

// Get 查询用户可见的一次执行。
func (uc *Usecase) Get(ctx context.Context, actorID, id string) (Execution, error) {
	return uc.store.GetExecution(ctx, actorID, id)
}

// ListSteps 返回一次执行已经持久化的模型和工具调用步骤。
func (uc *Usecase) ListSteps(ctx context.Context, actorID, executionID string) ([]Step, error) {
	return uc.store.ListExecutionSteps(ctx, actorID, executionID)
}

// Create 保存用户输入并创建排队中的执行任务。
func (uc *Usecase) Create(
	ctx context.Context,
	actorID string,
	conversationID string,
	userContent string,
) (Execution, error) {
	return uc.store.CreateExecution(ctx, actorID, conversationID, userContent)
}

// Cancel 取消排队任务，或通知当前执行实例停止模型调用。
func (uc *Usecase) Cancel(ctx context.Context, actorID, executionID string) (Execution, error) {
	execution, err := uc.store.CancelExecution(ctx, actorID, executionID)
	if err != nil {
		return Execution{}, err
	}
	if execution.State == StateCancelled {
		// MySQL 中的取消终态是最终事实；排队任务没有执行实例发事件，
		// 因此尝试用 Redis 事件提前唤醒订阅者，通知失败不得覆盖已提交的终态。
		_, _ = uc.events.Append(ctx, execution.ID, StreamEvent{Type: EventCancelled})
	}
	return execution, nil
}

// ReadEvents 读取指定事件之后的短期流式事件。
func (uc *Usecase) ReadEvents(
	ctx context.Context,
	executionID string,
	lastID string,
	limit int64,
	block time.Duration,
) ([]StreamEvent, error) {
	return uc.events.Read(ctx, executionID, lastID, limit, block)
}

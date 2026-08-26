package execution

import (
	"context"
	"time"
)

// Service 提供执行对象的创建、查询、取消和事件读取能力。
// 模型调用由 Executor 承担，API 请求不会同步执行耗时任务。
type Service struct {
	store  ServiceStore
	events EventStore
}

// NewService 创建面向 API 的执行服务。
func NewService(store ServiceStore, events EventStore) *Service {
	return &Service{store: store, events: events}
}

// Get 查询用户可见的一次执行。
func (s *Service) Get(ctx context.Context, actorID, id string) (Execution, error) {
	return s.store.GetExecution(ctx, actorID, id)
}

// ListSteps 返回一次执行已经持久化的模型和工具调用步骤。
func (s *Service) ListSteps(ctx context.Context, actorID, executionID string) ([]Step, error) {
	return s.store.ListExecutionSteps(ctx, actorID, executionID)
}

// Create 保存用户输入并创建排队中的执行任务。
func (s *Service) Create(
	ctx context.Context,
	actorID string,
	conversationID string,
	userContent string,
) (Execution, error) {
	return s.store.CreateExecution(ctx, actorID, conversationID, userContent)
}

// Cancel 取消排队任务，或通知当前执行实例停止模型调用。
func (s *Service) Cancel(ctx context.Context, actorID, executionID string) (Execution, error) {
	item, err := s.store.CancelExecution(ctx, actorID, executionID)
	if err != nil {
		return Execution{}, err
	}
	if item.State == StateCancelled {
		// 排队任务没有执行实例负责发事件，提交持久终态后主动唤醒订阅者。
		_, _ = s.events.Append(ctx, item.ID, StreamEvent{Type: EventCancelled})
	}
	return item, nil
}

// ReadEvents 读取指定事件之后的短期流式事件。
func (s *Service) ReadEvents(
	ctx context.Context,
	executionID string,
	lastID string,
	limit int64,
	block time.Duration,
) ([]StreamEvent, error) {
	return s.events.Read(ctx, executionID, lastID, limit, block)
}

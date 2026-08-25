package run

import (
	"context"
	"time"
)

// Service 管理 Run 的创建、查询、取消与后台执行。
type Service struct {
	store  Store
	events EventStore
	agent  Agent
}

// NewService 创建 Run 业务服务。
func NewService(store Store, events EventStore, agent Agent) *Service {
	return &Service{store: store, events: events, agent: agent}
}

// Get 查询用户可见的 Run。
func (s *Service) Get(ctx context.Context, actorID, id string) (Run, error) {
	return s.store.GetRun(ctx, actorID, id)
}

// ListItems 返回一次 Run 已经持久化的执行步骤。
func (s *Service) ListItems(ctx context.Context, actorID, runID string) ([]Item, error) {
	return s.store.ListRunItems(ctx, actorID, runID)
}

// Create 保存用户输入并创建排队中的 Run，模型调用由后台 Worker 异步执行。
func (s *Service) Create(
	ctx context.Context,
	actorID string,
	conversationID string,
	userContent string,
) (Run, error) {
	return s.store.CreateRun(ctx, actorID, conversationID, userContent)
}

// Cancel 取消排队中的 Run，或通知当前租约持有者停止正在执行的模型调用。
func (s *Service) Cancel(ctx context.Context, actorID, runID string) (Run, error) {
	item, err := s.store.CancelRun(ctx, actorID, runID)
	if err != nil {
		return Run{}, err
	}
	if item.State == StateCancelled {
		// 排队中的 Run 没有执行实例负责发事件，持久终态提交后主动唤醒订阅者。
		_, _ = s.events.Append(ctx, item.ID, StreamEvent{Type: EventCancelled})
	}
	return item, nil
}

// ReadEvents 读取指定事件之后的短期流式事件。
func (s *Service) ReadEvents(
	ctx context.Context,
	runID string,
	lastID string,
	limit int64,
	block time.Duration,
) ([]StreamEvent, error) {
	return s.events.Read(ctx, runID, lastID, limit, block)
}

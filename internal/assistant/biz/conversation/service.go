// Package conversation 实现运维助手的会话、消息和模型执行规则
package conversation

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

const defaultTitle = "新会话"

// Service 协调持久会话、Eino Agent 和短期事件流。
type Service struct {
	store  Store
	events EventStore
	agent  Agent
}

// NewService 创建会话业务服务。
func NewService(store Store, events EventStore, agent Agent) *Service {
	return &Service{store: store, events: events, agent: agent}
}

func (s *Service) Create(ctx context.Context, actorID, title string) (Conversation, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = defaultTitle
	}
	now := time.Now().UTC()
	return s.store.Create(ctx, Conversation{
		ID:        uuid.NewString(),
		ActorID:   actorID,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func (s *Service) Get(ctx context.Context, actorID, id string) (Conversation, error) {
	return s.store.Get(ctx, actorID, id)
}

func (s *Service) List(
	ctx context.Context,
	actorID string,
	limit int,
	cursor *ConversationCursor,
) (ConversationPage, error) {
	return s.store.List(ctx, actorID, limit, cursor)
}

func (s *Service) Delete(ctx context.Context, actorID, id string) error {
	return s.store.Delete(ctx, actorID, id)
}

func (s *Service) ListMessages(
	ctx context.Context,
	actorID string,
	conversationID string,
	cursor *MessageCursor,
	limit int,
) (MessagePage, error) {
	return s.store.ListMessages(ctx, actorID, conversationID, cursor, limit)
}

func (s *Service) GetRun(ctx context.Context, actorID, id string) (Run, error) {
	return s.store.GetRun(ctx, actorID, id)
}

// ListRunItems 返回一次 Run 已经持久化的执行步骤。
func (s *Service) ListRunItems(ctx context.Context, actorID, runID string) ([]RunItem, error) {
	return s.store.ListRunItems(ctx, actorID, runID)
}

// CreateRun 保存用户输入并创建排队中的 Run，模型调用由后台 Worker 异步执行。
func (s *Service) CreateRun(
	ctx context.Context,
	actorID string,
	conversationID string,
	userContent string,
) (Run, error) {
	return s.store.CreateRun(ctx, actorID, conversationID, userContent)
}

// CancelRun 取消排队中的 Run，或通知当前租约持有者停止正在执行的模型调用。
func (s *Service) CancelRun(ctx context.Context, actorID, runID string) (Run, error) {
	run, err := s.store.CancelRun(ctx, actorID, runID)
	if err != nil {
		return Run{}, err
	}
	if run.State == StateCancelled {
		// 排队中的 Run 没有执行实例负责发事件，业务层在持久终态提交后主动唤醒订阅者。
		_, _ = s.events.Append(ctx, run.ID, StreamEvent{Type: EventRunCancelled})
	}
	return run, nil
}

func (s *Service) ReadEvents(
	ctx context.Context,
	runID string,
	lastID string,
	limit int64,
	block time.Duration,
) ([]StreamEvent, error) {
	return s.events.Read(ctx, runID, lastID, limit, block)
}

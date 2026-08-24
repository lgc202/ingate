// Package conversation 实现运维助手的会话、消息和模型执行规则
package conversation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultTitle       = "新会话"
	eventTypeStarted   = "execution.started"
	eventTypeDelta     = "message.delta"
	eventTypeCompleted = "execution.completed"
	eventTypeFailed    = "execution.failed"
	failureModel       = "MODEL_UNAVAILABLE"
	failureModelConfig = "MODEL_NOT_CONFIGURED"
	failureClientGone  = "CLIENT_DISCONNECTED"
	failureEventStore  = "EVENT_STORE_UNAVAILABLE"
	failureTimeout     = 5 * time.Second
)

// Service 协调持久会话、Eino Agent 和短期事件流
type Service struct {
	store  Store
	events EventStore
	agent  Agent
}

// NewService 创建会话业务服务
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
		Version:   1,
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

func (s *Service) Delete(ctx context.Context, actorID, id string, version int64) error {
	return s.store.Delete(ctx, actorID, id, version)
}

func (s *Service) ListMessages(
	ctx context.Context,
	actorID string,
	conversationID string,
	afterSequence int64,
	limit int,
) (MessagePage, error) {
	return s.store.ListMessages(ctx, actorID, conversationID, afterSequence, limit)
}

func (s *Service) GetExecution(ctx context.Context, actorID, id string) (Execution, error) {
	return s.store.GetExecution(ctx, actorID, id)
}

// Chat 持久化用户输入并同步执行 Agent；所有分片先写入 Redis 再交给当前 SSE 连接
func (s *Service) Chat(
	ctx context.Context,
	actorID string,
	conversationID string,
	userContent string,
	emit func(StreamEvent) error,
) (Execution, error) {
	execution, err := s.store.BeginExecution(ctx, actorID, conversationID, userContent, s.agent.Model())
	if err != nil {
		return Execution{}, err
	}
	if err := s.publish(ctx, execution.ID, StreamEvent{Type: eventTypeStarted, Data: execution.ID}, emit); err != nil {
		return s.fail(ctx, execution, failureEventStore, err)
	}
	history, err := s.store.ListMessages(ctx, actorID, conversationID, 0, 200)
	if err != nil {
		return s.fail(ctx, execution, failureModel, fmt.Errorf("load conversation history: %w", err))
	}
	content, err := s.agent.Generate(ctx, history.Items, func(delta string) error {
		return s.publish(ctx, execution.ID, StreamEvent{Type: eventTypeDelta, Data: delta}, emit)
	})
	if err != nil {
		failureCode := failureModel
		if errors.Is(err, ErrModelNotConfigured) {
			failureCode = failureModelConfig
		} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			failureCode = failureClientGone
		}
		return s.fail(ctx, execution, failureCode, err)
	}
	message, err := s.store.CompleteExecution(ctx, actorID, execution.ID, content)
	if err != nil {
		return s.fail(ctx, execution, failureModel, fmt.Errorf("complete execution: %w", err))
	}
	execution.State = StateSucceeded
	if err := s.publish(ctx, execution.ID, StreamEvent{Type: eventTypeCompleted, Data: message.ID}, emit); err != nil {
		return execution, err
	}
	return execution, nil
}

func (s *Service) ReadEvents(
	ctx context.Context,
	executionID string,
	lastID string,
	limit int64,
	block time.Duration,
) ([]StreamEvent, error) {
	return s.events.Read(ctx, executionID, lastID, limit, block)
}

func (s *Service) publish(
	ctx context.Context,
	executionID string,
	event StreamEvent,
	emit func(StreamEvent) error,
) error {
	id, err := s.events.Append(ctx, executionID, event)
	if err != nil {
		return fmt.Errorf("append execution event: %w", err)
	}
	event.ID = id
	if err := emit(event); err != nil {
		return fmt.Errorf("emit execution event: %w", err)
	}
	return nil
}

func (s *Service) fail(ctx context.Context, execution Execution, code string, cause error) (Execution, error) {
	// 客户端断开会取消请求 Context，但已经创建的执行仍必须落到终态。
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureTimeout)
	defer cancel()
	execution.State = StateFailed
	execution.FailureCode = code
	finishedAt := time.Now().UTC()
	execution.FinishedAt = &finishedAt
	if err := s.store.FailExecution(cleanupCtx, execution.ID, code); err != nil {
		return execution, errors.Join(cause, fmt.Errorf("mark execution failed: %w", err))
	}
	// 失败事件写入失败不能覆盖原始执行错误；重连仍可从 MySQL 读取失败状态
	_, _ = s.events.Append(cleanupCtx, execution.ID, StreamEvent{Type: eventTypeFailed, Data: code})
	return execution, cause
}

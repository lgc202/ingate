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
	eventTypeStarted   = "run.started"
	eventTypeReasoning = "message.reasoning.delta"
	eventTypeContent   = "message.content.delta"
	eventTypeCompleted = "run.completed"
	eventTypeFailed    = "run.failed"
	failureInternal    = "INTERNAL_ERROR"
	failureModel       = "MODEL_UNAVAILABLE"
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

// Chat 持久化用户输入并同步执行 Agent；所有分片先写入 Redis 再交给当前 SSE 连接
func (s *Service) Chat(
	ctx context.Context,
	actorID string,
	conversationID string,
	userContent string,
	emit func(StreamEvent) error,
) (Run, error) {
	selectedModel, err := s.agent.Model(ctx)
	if err != nil {
		return Run{}, err
	}
	run, err := s.store.BeginRun(ctx, actorID, conversationID, userContent, selectedModel.Name())
	if err != nil {
		return Run{}, err
	}
	if err := s.publish(ctx, run.ID, StreamEvent{Type: eventTypeStarted, Data: run.ID}, emit); err != nil {
		return s.fail(ctx, actorID, run, runFailureCode(err), err)
	}
	history, err := s.store.ListRecentMessages(ctx, actorID, conversationID, 200)
	if err != nil {
		return s.fail(ctx, actorID, run, failureInternal, fmt.Errorf("load conversation history: %w", err))
	}
	result, err := selectedModel.Generate(ctx, history, func(delta ModelDelta) error {
		eventType := eventTypeContent
		if delta.Type == ModelDeltaReasoning {
			eventType = eventTypeReasoning
		}
		return s.publish(ctx, run.ID, StreamEvent{Type: eventType, Data: delta.Content}, emit)
	})
	if err != nil {
		return s.fail(ctx, actorID, run, runFailureCode(err), err)
	}
	message, err := s.store.CompleteRun(ctx, actorID, run.ID, result)
	if err != nil {
		return s.fail(ctx, actorID, run, failureInternal, fmt.Errorf("complete assistant run: %w", err))
	}
	run.State = StateSucceeded
	event := StreamEvent{Type: eventTypeCompleted, Data: message.ID}
	eventID, err := s.events.Append(ctx, run.ID, event)
	if err != nil {
		return run, fmt.Errorf("append completed assistant run event: %w", err)
	}
	event.ID = eventID
	// Run 已经持久化为成功，连接在最后一个分片后断开不能把成功结果改写成失败；
	// 客户端可以使用 Last-Event-ID 从 Redis 重新读取完成事件。
	_ = emit(event)
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

func (s *Service) publish(
	ctx context.Context,
	runID string,
	event StreamEvent,
	emit func(StreamEvent) error,
) error {
	id, err := s.events.Append(ctx, runID, event)
	if err != nil {
		return errors.Join(errEventStoreUnavailable, fmt.Errorf("append assistant run event: %w", err))
	}
	event.ID = id
	if err := emit(event); err != nil {
		return errors.Join(errStreamDisconnected, fmt.Errorf("emit assistant run event: %w", err))
	}
	return nil
}

func (s *Service) fail(ctx context.Context, actorID string, run Run, code string, cause error) (Run, error) {
	// 客户端断开会取消请求 Context，但已经创建的 Run 仍必须落到终态。
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureTimeout)
	defer cancel()
	run.State = StateFailed
	run.ErrorCode = code
	finishedAt := time.Now().UTC()
	run.FinishedAt = &finishedAt
	if err := s.store.FailRun(cleanupCtx, actorID, run.ID, code); err != nil {
		return run, errors.Join(cause, fmt.Errorf("mark assistant run failed: %w", err))
	}
	// 失败事件写入失败不能覆盖原始 Run 错误；重连仍可从 MySQL 读取最终状态。
	_, _ = s.events.Append(cleanupCtx, run.ID, StreamEvent{Type: eventTypeFailed, Data: code})
	return run, cause
}

func runFailureCode(err error) string {
	switch {
	case errors.Is(err, errEventStoreUnavailable):
		return failureEventStore
	case errors.Is(err, errStreamDisconnected),
		errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return failureClientGone
	default:
		return failureModel
	}
}

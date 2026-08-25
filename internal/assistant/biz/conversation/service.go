// Package conversation 实现运维助手的会话和消息规则。
package conversation

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

const defaultTitle = "新会话"

// Service 管理持久会话与消息查询。
type Service struct {
	store Store
}

// NewService 创建会话业务服务。
func NewService(store Store) *Service {
	return &Service{store: store}
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

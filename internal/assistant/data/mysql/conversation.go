package mysql

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

// 消息角色在数据库中使用小整数保存，领域层继续使用可读字符串。
const (
	messageRoleUser uint8 = iota + 1
	messageRoleAssistant
)

// Create 持久化一个已由业务层完成校验和初始化的会话。
func (s *Store) Create(ctx context.Context, item conversation.Conversation) (conversation.Conversation, error) {
	err := s.queries.CreateConversation(ctx, db.CreateConversationParams{
		ID:        item.ID,
		ActorID:   item.ActorID,
		Title:     item.Title,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	})
	if err != nil {
		return conversation.Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	return item, nil
}

// Get 按所有者和会话 ID 查询会话，避免跨用户读取。
func (s *Store) Get(ctx context.Context, actorID, id string) (conversation.Conversation, error) {
	item, err := s.queries.GetConversation(ctx, db.GetConversationParams{ID: id, ActorID: actorID})
	if err != nil {
		return conversation.Conversation{}, mapNotFound(err)
	}
	return conversationFromDB(item), nil
}

// List 使用 updated_at 和 id 组成稳定游标，避免会话活跃度相同时漏项。
func (s *Store) List(
	ctx context.Context,
	actorID string,
	limit int,
	cursor *conversation.ConversationCursor,
) (conversation.ConversationPage, error) {
	updatedAt := time.Date(9999, 12, 31, 23, 59, 59, 999999000, time.UTC)
	id := "~"
	if cursor != nil {
		updatedAt = cursor.UpdatedAt
		id = cursor.ID
	}
	rows, err := s.queries.ListConversations(ctx, db.ListConversationsParams{
		ActorID:     actorID,
		UpdatedAt:   updatedAt,
		UpdatedAt_2: updatedAt,
		ID:          id,
		Limit:       int32(limit + 1),
	})
	if err != nil {
		return conversation.ConversationPage{}, fmt.Errorf("list conversations: %w", err)
	}
	page := conversation.ConversationPage{Items: make([]conversation.Conversation, 0, min(len(rows), limit))}
	for _, row := range rows[:min(len(rows), limit)] {
		page.Items = append(page.Items, conversationFromDB(row))
	}
	if len(rows) > limit {
		last := page.Items[len(page.Items)-1]
		page.NextCursor = &conversation.ConversationCursor{UpdatedAt: last.UpdatedAt, ID: last.ID}
	}
	return page, nil
}

// Delete 在没有排队或运行中的 Run 时删除整个会话聚合。
func (s *Store) Delete(ctx context.Context, actorID, id string) error {
	err := s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: id, ActorID: actorID,
		}); err != nil {
			return mapNotFound(err)
		}
		active, err := queries.CountActiveRuns(ctx, id)
		if err != nil {
			return fmt.Errorf("count active assistant runs: %w", err)
		}
		if active > 0 {
			return conversation.ErrRunRunning
		}

		// 数据库不使用外键级联；聚合删除必须在同一事务中按依赖顺序显式完成。
		if err := queries.DeleteMessagesByConversation(ctx, id); err != nil {
			return fmt.Errorf("delete conversation messages: %w", err)
		}
		if err := queries.DeleteRunsByConversation(ctx, id); err != nil {
			return fmt.Errorf("delete conversation runs: %w", err)
		}
		rows, err := queries.DeleteConversation(ctx, db.DeleteConversationParams{ID: id, ActorID: actorID})
		if err != nil {
			return fmt.Errorf("delete conversation: %w", err)
		}
		if rows != 1 {
			return conversation.ErrNotFound
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("delete conversation transaction: %w", err)
	}
	return nil
}

// ListMessages 按创建时间正序分页返回会话的持久消息。
func (s *Store) ListMessages(
	ctx context.Context,
	actorID string,
	conversationID string,
	cursor *conversation.MessageCursor,
	limit int,
) (conversation.MessagePage, error) {
	if _, err := s.Get(ctx, actorID, conversationID); err != nil {
		return conversation.MessagePage{}, err
	}
	createdAt := time.Unix(0, 0).UTC()
	id := ""
	if cursor != nil {
		createdAt = cursor.CreatedAt
		id = cursor.ID
	}
	rows, err := s.queries.ListMessages(ctx, db.ListMessagesParams{
		ConversationID: conversationID,
		CreatedAt:      createdAt,
		CreatedAt_2:    createdAt,
		ID:             id,
		Limit:          int32(limit + 1),
	})
	if err != nil {
		return conversation.MessagePage{}, fmt.Errorf("list conversation messages: %w", err)
	}
	page := conversation.MessagePage{Items: make([]conversation.Message, 0, min(len(rows), limit))}
	for _, row := range rows[:min(len(rows), limit)] {
		message, err := messageFromDB(row)
		if err != nil {
			return conversation.MessagePage{}, fmt.Errorf("decode conversation message: %w", err)
		}
		page.Items = append(page.Items, message)
	}
	if len(rows) > limit {
		last := page.Items[len(page.Items)-1]
		page.NextCursor = &conversation.MessageCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return page, nil
}

// ListRecentMessages 返回模型上下文需要的最近消息，并恢复为时间正序。
func (s *Store) ListRecentMessages(
	ctx context.Context,
	actorID string,
	conversationID string,
	limit int,
) ([]conversation.Message, error) {
	if _, err := s.Get(ctx, actorID, conversationID); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListRecentMessages(ctx, db.ListRecentMessagesParams{
		ConversationID: conversationID,
		Limit:          int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list recent conversation messages: %w", err)
	}
	messages := make([]conversation.Message, 0, len(rows))
	for _, row := range rows {
		message, err := messageFromDB(row)
		if err != nil {
			return nil, fmt.Errorf("decode recent conversation message: %w", err)
		}
		messages = append(messages, message)
	}
	slices.Reverse(messages)
	return messages, nil
}

func conversationFromDB(item db.AssistantConversation) conversation.Conversation {
	return conversation.Conversation{
		ID:        item.ID,
		ActorID:   item.ActorID,
		Title:     item.Title,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func messageFromDB(item db.AssistantMessage) (conversation.Message, error) {
	role, err := messageRoleFromDB(item.Role)
	if err != nil {
		return conversation.Message{}, err
	}
	return conversation.Message{
		ID:               item.ID,
		ConversationID:   item.ConversationID,
		RunID:            item.RunID,
		Role:             role,
		Content:          item.Content,
		ReasoningContent: item.ReasoningContent,
		CreatedAt:        item.CreatedAt,
	}, nil
}

func messageRoleFromDB(role uint8) (conversation.MessageRole, error) {
	switch role {
	case messageRoleUser:
		return conversation.RoleUser, nil
	case messageRoleAssistant:
		return conversation.RoleAssistant, nil
	default:
		return "", fmt.Errorf("invalid assistant message role %d", role)
	}
}

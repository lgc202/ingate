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

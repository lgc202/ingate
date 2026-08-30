package mysql

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

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
			return conversation.MessagePage{}, fmt.Errorf("restore conversation message: %w", err)
		}
		page.Items = append(page.Items, message)
	}
	if len(rows) > limit {
		last := page.Items[len(page.Items)-1]
		page.NextCursor = &conversation.MessageCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return page, nil
}

// ListRecentMessages 返回模型上下文需要的最近消息，并同时限制条数和正文总量。
// 第一次查询只读取长度，避免为了判断预算先把大段模型回复载入进程。
func (s *Store) ListRecentMessages(
	ctx context.Context,
	actorID string,
	conversationID string,
	maxMessages int,
	maxContentBytes int64,
) ([]conversation.HistoryMessage, error) {
	if _, err := s.Get(ctx, actorID, conversationID); err != nil {
		return nil, err
	}
	sizes, err := s.queries.ListRecentMessageSizes(ctx, db.ListRecentMessageSizesParams{
		ConversationID: conversationID,
		Limit:          int32(maxMessages),
	})
	if err != nil {
		return nil, fmt.Errorf("list recent conversation message sizes: %w", err)
	}
	if len(sizes) == 0 {
		return nil, errors.New("conversation history is empty")
	}
	remaining := maxContentBytes
	messageCount := 0
	for _, size := range sizes {
		if size <= 0 {
			return nil, errors.New("conversation history contains an empty message")
		}
		if int64(size) > remaining {
			break
		}
		remaining -= int64(size)
		messageCount++
	}
	if messageCount == 0 {
		return nil, errors.New("latest conversation message exceeds the history size limit")
	}
	rows, err := s.queries.ListRecentMessages(ctx, db.ListRecentMessagesParams{
		ConversationID: conversationID,
		Limit:          int32(messageCount),
	})
	if err != nil {
		return nil, fmt.Errorf("list recent conversation messages: %w", err)
	}
	if len(rows) != messageCount {
		return nil, errors.New("conversation history changed while it was being read")
	}
	messages := make([]conversation.HistoryMessage, 0, len(rows))
	var contentBytes int64
	for _, row := range rows {
		role, err := messageRoleFromDB(row.Role)
		if err != nil {
			return nil, fmt.Errorf("restore recent conversation message: %w", err)
		}
		if row.Content == "" {
			return nil, errors.New("conversation history contains an empty message")
		}
		contentBytes += int64(len(row.Content))
		if contentBytes > maxContentBytes {
			return nil, errors.New("conversation history exceeds the size limit")
		}
		messages = append(messages, conversation.HistoryMessage{
			Role:    role,
			Content: row.Content,
		})
	}
	slices.Reverse(messages)
	return messages, nil
}

func messageFromDB(item db.AssistantMessage) (conversation.Message, error) {
	if uuid.Validate(item.ID) != nil || uuid.Validate(item.ConversationID) != nil ||
		uuid.Validate(item.ExecutionID) != nil || item.Content == "" || item.CreatedAt.IsZero() {
		return conversation.Message{}, fmt.Errorf("invalid stored assistant message %q", item.ID)
	}
	role, err := messageRoleFromDB(item.Role)
	if err != nil {
		return conversation.Message{}, err
	}
	if role == conversation.RoleUser && item.ReasoningContent != "" {
		return conversation.Message{}, fmt.Errorf("user message %s contains reasoning content", item.ID)
	}
	return conversation.Message{
		ID:               item.ID,
		ConversationID:   item.ConversationID,
		ExecutionID:      item.ExecutionID,
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

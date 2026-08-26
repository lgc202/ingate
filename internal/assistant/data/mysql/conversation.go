package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	runbiz "github.com/lgc202/ingate/internal/assistant/biz/run"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
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
		return conversation.Conversation{}, mapConversationNotFound(err)
	}
	return conversationFromDB(item), nil
}

// UpdateTitle 修改属于当前调用方的会话名称，并返回数据库中的最终状态。
func (s *Store) UpdateTitle(
	ctx context.Context,
	actorID string,
	id string,
	title string,
	updatedAt time.Time,
) (conversation.Conversation, error) {
	rows, err := s.queries.UpdateConversationTitle(ctx, db.UpdateConversationTitleParams{
		Title:     title,
		UpdatedAt: updatedAt,
		ID:        id,
		ActorID:   actorID,
	})
	if err != nil {
		return conversation.Conversation{}, fmt.Errorf("update conversation title: %w", err)
	}
	if rows != 1 {
		return conversation.Conversation{}, conversation.ErrNotFound
	}
	return s.Get(ctx, actorID, id)
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
			return mapConversationNotFound(err)
		}
		active, err := queries.CountActiveRuns(ctx, id)
		if err != nil {
			return fmt.Errorf("count active assistant runs: %w", err)
		}
		if active > 0 {
			return runbiz.ErrConversationBusy
		}

		// 数据库不使用外键级联；聚合删除必须在同一事务中按依赖顺序显式完成。
		if err := queries.DeleteMessagesByConversation(ctx, id); err != nil {
			return fmt.Errorf("delete conversation messages: %w", err)
		}
		if err := queries.DeleteRunItemsByConversation(ctx, id); err != nil {
			return fmt.Errorf("delete conversation run items: %w", err)
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

func conversationFromDB(item db.AssistantConversation) conversation.Conversation {
	return conversation.Conversation{
		ID:        item.ID,
		ActorID:   item.ActorID,
		Title:     item.Title,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

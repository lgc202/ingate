package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	"github.com/lgc202/ingate/internal/assistant/biz/execution"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
	"github.com/lgc202/ingate/internal/pkg/adminidentity"
)

// Create 使用数据库时钟持久化一个已由业务层完成校验的会话。
func (s *Store) Create(ctx context.Context, actorID, title string) (conversation.Conversation, error) {
	now, err := s.queries.CurrentTime(ctx)
	if err != nil {
		return conversation.Conversation{}, fmt.Errorf("read MySQL time: %w", err)
	}
	item := conversation.Conversation{
		ID:        uuid.NewString(),
		ActorID:   actorID,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.queries.CreateConversation(ctx, db.CreateConversationParams{
		ID:        item.ID,
		ActorID:   item.ActorID,
		Title:     item.Title,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}); err != nil {
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
	result, err := conversationFromDB(item)
	if err != nil {
		return conversation.Conversation{}, fmt.Errorf("restore conversation: %w", err)
	}
	return result, nil
}

// UpdateTitle 修改属于当前调用方的会话名称，并返回数据库中的最终状态。
func (s *Store) UpdateTitle(
	ctx context.Context,
	actorID string,
	id string,
	title string,
) (conversation.Conversation, error) {
	rows, err := s.queries.UpdateConversationTitle(ctx, db.UpdateConversationTitleParams{
		Title:   title,
		ID:      id,
		ActorID: actorID,
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
		item, err := conversationFromDB(row)
		if err != nil {
			return conversation.ConversationPage{}, fmt.Errorf("restore conversation: %w", err)
		}
		page.Items = append(page.Items, item)
	}
	if len(rows) > limit {
		last := page.Items[len(page.Items)-1]
		page.NextCursor = &conversation.ConversationCursor{UpdatedAt: last.UpdatedAt, ID: last.ID}
	}
	return page, nil
}

// Delete 在没有排队或运行中的 Agent 执行时删除整个会话聚合。
func (s *Store) Delete(ctx context.Context, actorID, id string) error {
	err := s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: id, ActorID: actorID,
		}); err != nil {
			return mapConversationNotFound(err)
		}
		active, err := queries.CountActiveExecutions(ctx, id)
		if err != nil {
			return fmt.Errorf("count active assistant executions: %w", err)
		}
		if active > 0 {
			return execution.ErrConversationBusy
		}

		// 数据库不使用外键级联；聚合删除必须在同一事务中按依赖顺序显式完成。
		if err := queries.DeleteMessagesByConversation(ctx, id); err != nil {
			return fmt.Errorf("delete conversation messages: %w", err)
		}
		if err := queries.DeleteExecutionStepsByConversation(ctx, id); err != nil {
			return fmt.Errorf("delete conversation execution steps: %w", err)
		}
		if err := queries.DeleteExecutionsByConversation(ctx, id); err != nil {
			return fmt.Errorf("delete conversation executions: %w", err)
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

func conversationFromDB(item db.AssistantConversation) (conversation.Conversation, error) {
	if uuid.Validate(item.ID) != nil || !adminidentity.IsValid(item.ActorID) ||
		!conversation.IsValidTitle(item.Title) || item.CreatedAt.IsZero() ||
		item.UpdatedAt.Before(item.CreatedAt) {
		return conversation.Conversation{}, fmt.Errorf("invalid stored conversation %q", item.ID)
	}
	return conversation.Conversation{
		ID:        item.ID,
		ActorID:   item.ActorID,
		Title:     item.Title,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}, nil
}

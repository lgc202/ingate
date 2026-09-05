// Package conversation 实现运维助手的会话和消息规则。
package conversation

import (
	"cmp"
	"context"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	defaultTitle  = "新会话"
	maxTitleRunes = 160
)

// Store 由会话业务定义持久化边界，MySQL 实现事务和并发约束。
type Store interface {
	Create(ctx context.Context, actorID, title string) (Conversation, error)
	Get(context.Context, string, string) (Conversation, error)
	UpdateTitle(ctx context.Context, actorID, id, title string) (Conversation, error)
	List(context.Context, string, int, *Cursor) (Page, error)
	Delete(context.Context, string, string) error
	ListMessages(context.Context, string, string, *MessageCursor, int) (MessagePage, error)
}

// Usecase 管理持久会话与消息查询。
type Usecase struct {
	store Store
}

// NewUsecase 创建会话业务入口。
func NewUsecase(store Store) *Usecase {
	return &Usecase{store: store}
}

// Create 创建属于指定管理员的会话，并为空标题生成默认名称。
func (uc *Usecase) Create(ctx context.Context, actorID, title string) (Conversation, error) {
	title = strings.TrimSpace(title)
	title = cmp.Or(title, defaultTitle)
	if !IsValidTitle(title) {
		return Conversation{}, ErrInvalidTitle
	}
	return uc.store.Create(ctx, actorID, title)
}

// Get 返回指定管理员可见的单个会话。
func (uc *Usecase) Get(ctx context.Context, actorID, id string) (Conversation, error) {
	return uc.store.Get(ctx, actorID, id)
}

// UpdateTitle 更新指定管理员会话的非空标题。
func (uc *Usecase) UpdateTitle(ctx context.Context, actorID, id, title string) (Conversation, error) {
	title = strings.TrimSpace(title)
	if !IsValidTitle(title) {
		return Conversation{}, ErrInvalidTitle
	}
	return uc.store.UpdateTitle(ctx, actorID, id, title)
}

// List 按更新时间倒序返回指定管理员的会话。
func (uc *Usecase) List(
	ctx context.Context,
	actorID string,
	limit int,
	cursor *Cursor,
) (Page, error) {
	return uc.store.List(ctx, actorID, limit, cursor)
}

// Delete 删除指定管理员的会话及其关联记录。
func (uc *Usecase) Delete(ctx context.Context, actorID, id string) error {
	return uc.store.Delete(ctx, actorID, id)
}

// ListMessages 按创建时间返回指定会话中已经持久化的消息。
func (uc *Usecase) ListMessages(
	ctx context.Context,
	actorID string,
	conversationID string,
	cursor *MessageCursor,
	limit int,
) (MessagePage, error) {
	return uc.store.ListMessages(ctx, actorID, conversationID, cursor, limit)
}

// IsValidTitle 判断 title 是否可以作为会话展示标题持久化。
func IsValidTitle(title string) bool {
	if title == "" || !utf8.ValidString(title) || utf8.RuneCountInString(title) > maxTitleRunes {
		return false
	}
	for _, character := range title {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

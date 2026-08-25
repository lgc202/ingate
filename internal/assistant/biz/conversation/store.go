package conversation

import (
	"context"
)

// Store 由会话业务定义持久化边界，MySQL 实现事务和并发约束。
type Store interface {
	Create(context.Context, Conversation) (Conversation, error)
	Get(context.Context, string, string) (Conversation, error)
	List(context.Context, string, int, *ConversationCursor) (ConversationPage, error)
	Delete(context.Context, string, string) error
	ListMessages(context.Context, string, string, *MessageCursor, int) (MessagePage, error)
}

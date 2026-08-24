package conversation

import (
	"context"
	"time"
)

// Store 由业务层定义持久化边界，MySQL 实现事务和并发约束
type Store interface {
	Create(context.Context, Conversation) (Conversation, error)
	Get(context.Context, string, string) (Conversation, error)
	List(context.Context, string, int, *ConversationCursor) (ConversationPage, error)
	Delete(context.Context, string, string, int64) error
	ListMessages(context.Context, string, string, int64, int) (MessagePage, error)
	BeginExecution(context.Context, string, string, string, string) (Execution, error)
	CompleteExecution(context.Context, string, string, string) (Message, error)
	FailExecution(context.Context, string, string) error
	GetExecution(context.Context, string, string) (Execution, error)
}

// EventStore 保存可过期的执行事件，不能作为消息或执行状态的事实来源
type EventStore interface {
	Append(context.Context, string, StreamEvent) (string, error)
	Read(context.Context, string, string, int64, time.Duration) ([]StreamEvent, error)
}

// Agent 隔离 Eino 的消息和流式类型，业务层只依赖项目领域对象
type Agent interface {
	Model() string
	Generate(context.Context, []Message, func(string) error) (string, error)
}

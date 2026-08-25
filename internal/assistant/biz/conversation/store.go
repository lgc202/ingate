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
	Delete(context.Context, string, string) error
	ListMessages(context.Context, string, string, *MessageCursor, int) (MessagePage, error)
	ListRecentMessages(context.Context, string, string, int) ([]Message, error)
	BeginRun(context.Context, string, string, string, string) (Run, error)
	CompleteRun(context.Context, string, string, ModelResult) (Message, error)
	FailRun(context.Context, string, string, string) error
	GetRun(context.Context, string, string) (Run, error)
}

// EventStore 保存可过期的 Run 事件，不能作为消息或 Run 状态的事实来源。
type EventStore interface {
	Append(context.Context, string, StreamEvent) (string, error)
	Read(context.Context, string, string, int64, time.Duration) ([]StreamEvent, error)
}

// Model 是一次 Run 固定使用的模型快照。
type Model interface {
	Name() string
	Generate(context.Context, []Message, func(ModelDelta) error) (ModelResult, error)
}

// Agent 隔离 Eino 的消息和流式类型，业务层只依赖项目领域对象。
type Agent interface {
	Model(context.Context) (Model, error)
}

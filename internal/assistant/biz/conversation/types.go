package conversation

import "time"

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"

	StateRunning   = "running"
	StateSucceeded = "succeeded"
	StateFailed    = "failed"
)

// Conversation 表示一个管理员与运维助手的持久会话
type Conversation struct {
	ID        string
	ActorID   string
	Title     string
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message 是会话中按 sequence 严格排序的一条消息
type Message struct {
	ID             string
	ConversationID string
	Sequence       int64
	Role           string
	Content        string
	CreatedAt      time.Time
}

// Execution 记录一次模型执行的最终状态，不保存短期流式分片
type Execution struct {
	ID             string
	ConversationID string
	State          string
	Model          string
	FailureCode    string
	StartedAt      time.Time
	FinishedAt     *time.Time
}

// ConversationCursor 是按更新时间倒序翻页的稳定游标
type ConversationCursor struct {
	UpdatedAt time.Time
	ID        string
}

// ConversationPage 是一次会话分页查询结果
type ConversationPage struct {
	Items      []Conversation
	NextCursor *ConversationCursor
}

// MessagePage 是一次消息分页查询结果
type MessagePage struct {
	Items        []Message
	NextSequence int64
}

// StreamEvent 是可通过 SSE 重放的单次执行事件
type StreamEvent struct {
	ID   string
	Type string
	Data string
}

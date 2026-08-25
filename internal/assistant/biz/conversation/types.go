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
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message 是一次 Run 产生并归属于会话的一条持久消息。
type Message struct {
	ID               string
	ConversationID   string
	RunID            string
	Role             string
	Content          string
	ReasoningContent string
	CreatedAt        time.Time
}

// ModelDeltaType 区分模型流中的推理内容和最终回答。
type ModelDeltaType uint8

const (
	ModelDeltaReasoning ModelDeltaType = iota + 1
	ModelDeltaContent
)

// ModelDelta 是模型返回的一段增量内容。
type ModelDelta struct {
	Type    ModelDeltaType
	Content string
}

// ModelResult 是一次模型调用需要持久化的最终结果。
type ModelResult struct {
	Content          string
	ReasoningContent string
}

// Run 记录一次用户输入从接收到完成的生命周期，不保存短期流式分片。
type Run struct {
	ID             string
	ConversationID string
	State          string
	Model          string
	ErrorCode      string
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

// MessageCursor 以创建时间和 ID 唯一确定消息分页位置。
type MessageCursor struct {
	CreatedAt time.Time
	ID        string
}

// MessagePage 是一次消息分页查询结果
type MessagePage struct {
	Items      []Message
	NextCursor *MessageCursor
}

// StreamEvent 是可通过 SSE 重放的单次 Run 事件。
type StreamEvent struct {
	ID   string
	Type string
	Data string
}

package conversation

import "time"

// MessageRole 表示一条持久消息在对话中的职责。
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
)

// Conversation 表示一个管理员与运维助手的持久会话。
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
	Role             MessageRole
	Content          string
	ReasoningContent string
	CreatedAt        time.Time
}

// ConversationCursor 是按更新时间倒序翻页的稳定游标。
type ConversationCursor struct {
	UpdatedAt time.Time
	ID        string
}

// ConversationPage 是一次会话分页查询结果。
type ConversationPage struct {
	Items      []Conversation
	NextCursor *ConversationCursor
}

// MessageCursor 以创建时间和 ID 唯一确定消息分页位置。
type MessageCursor struct {
	CreatedAt time.Time
	ID        string
}

// MessagePage 是一次消息分页查询结果。
type MessagePage struct {
	Items      []Message
	NextCursor *MessageCursor
}

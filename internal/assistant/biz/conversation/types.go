package conversation

import (
	"errors"
	"time"
)

const (
	// RoleUser 表示管理员发送的持久消息。
	RoleUser MessageRole = "user"
	// RoleAssistant 表示 Agent 生成的持久消息。
	RoleAssistant MessageRole = "assistant"
)

var (
	// ErrNotFound 表示会话或其消息对当前管理员不可见。
	ErrNotFound = errors.New("conversation resource not found")
	// ErrInvalidTitle 表示会话标题为空或不符合展示文本约束。
	ErrInvalidTitle = errors.New("conversation title is invalid")
)

// MessageRole 表示一条持久消息在对话中的职责。
type MessageRole string

// Conversation 表示一个管理员与运维助手的持久会话。
type Conversation struct {
	ID        string
	ActorID   string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message 是一次 Agent 执行产生并归属于会话的持久消息。
type Message struct {
	ID               string
	ConversationID   string
	ExecutionID      string
	Role             MessageRole
	Content          string
	ReasoningContent string
	CreatedAt        time.Time
}

// HistoryMessage 是一次新执行恢复模型上下文时需要的最小持久消息视图。
// 推理内容、标识和时间不会重新发送给模型。
type HistoryMessage struct {
	Role    MessageRole
	Content string
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

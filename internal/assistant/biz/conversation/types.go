package conversation

import "time"

// MessageRole 表示一条持久消息在对话中的职责。
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
)

// RunState 表示一次助手执行的持久状态。
type RunState string

const (
	StateQueued    RunState = "queued"
	StateRunning   RunState = "running"
	StateSucceeded RunState = "succeeded"
	StateFailed    RunState = "failed"
	StateCancelled RunState = "cancelled"
)

// EventType 是浏览器可订阅和短时重放的流式事件类型。
type EventType string

const (
	EventRunStarted     EventType = "run.started"
	EventReasoningDelta EventType = "message.reasoning.delta"
	EventContentDelta   EventType = "message.content.delta"
	EventRunCompleted   EventType = "run.completed"
	EventRunFailed      EventType = "run.failed"
	EventRunCancelled   EventType = "run.cancelled"
)

// FailureCode 是持久化到 Run 并可安全返回客户端的稳定错误码。
type FailureCode string

const (
	FailureInternal         FailureCode = "INTERNAL_ERROR"
	FailureModelUnavailable FailureCode = "MODEL_UNAVAILABLE"
	FailureEventStore       FailureCode = "EVENT_STORE_UNAVAILABLE"
	FailureWorkerLost       FailureCode = "WORKER_LOST"
	FailureWorkerStopped    FailureCode = "WORKER_STOPPED"
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
	ID                    string
	ConversationID        string
	State                 RunState
	Model                 string
	ErrorCode             FailureCode
	CancellationRequested bool
	CreatedAt             time.Time
	StartedAt             *time.Time
	FinishedAt            *time.Time
}

// ClaimedRun 是后台实例已取得租约、可以执行的一次 Run。
// ActorID 只用于读取所属用户的数据，不会写入事件或返回客户端。
type ClaimedRun struct {
	Run
	ActorID string
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

// StreamEvent 是可通过 SSE 重放的单次 Run 事件。
type StreamEvent struct {
	ID   string
	Type EventType
	Data string
}

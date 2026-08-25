// Package run 实现运维助手一次异步执行的状态、租约和事件规则。
package run

import "time"

// State 表示一次助手执行的持久状态。
type State string

const (
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
	StateCancelled State = "cancelled"
)

// ItemKind 表示一次 Run 中可持久追踪的执行步骤类型。
type ItemKind string

const (
	ItemKindModelCall ItemKind = "model_call"
	ItemKindToolCall  ItemKind = "tool_call"
)

// ItemState 表示执行步骤自身的生命周期，不替代 Run 的整体状态。
type ItemState string

const (
	ItemStateRunning   ItemState = "running"
	ItemStateCompleted ItemState = "completed"
	ItemStateFailed    ItemState = "failed"
	ItemStateCancelled ItemState = "cancelled"
)

// EventType 是浏览器可订阅和短时重放的流式事件类型。
type EventType string

const (
	EventStarted        EventType = "run.started"
	EventReasoningDelta EventType = "message.reasoning.delta"
	EventContentDelta   EventType = "message.content.delta"
	EventCompleted      EventType = "run.completed"
	EventFailed         EventType = "run.failed"
	EventCancelled      EventType = "run.cancelled"
	EventStreamFailed   EventType = "stream.failed"
)

// FailureCode 是持久化到 Run 并可安全返回客户端的稳定错误码。
type FailureCode string

const (
	FailureInternal         FailureCode = "INTERNAL_ERROR"
	FailureModelUnavailable FailureCode = "MODEL_UNAVAILABLE"
	FailureToolUnavailable  FailureCode = "TOOL_UNAVAILABLE"
	FailureEventStore       FailureCode = "EVENT_STORE_UNAVAILABLE"
	FailureWorkerLost       FailureCode = "WORKER_LOST"
	FailureWorkerStopped    FailureCode = "WORKER_STOPPED"
)

// DeltaType 区分模型流中的推理内容和最终回答。
type DeltaType uint8

const (
	DeltaReasoning DeltaType = iota + 1
	DeltaContent
)

// Delta 是模型返回的一段增量内容。
type Delta struct {
	Type    DeltaType
	Content string
}

// Result 是一次模型调用需要持久化的最终结果。
type Result struct {
	Content          string
	ReasoningContent string
}

// Run 记录一次用户输入从接收到完成的生命周期，不保存短期流式分片。
type Run struct {
	ID                    string
	ConversationID        string
	State                 State
	Model                 string
	ErrorCode             FailureCode
	CancellationRequested bool
	CreatedAt             time.Time
	StartedAt             *time.Time
	FinishedAt            *time.Time
}

// Item 记录一次 Run 中的模型或工具调用。
// Summary 只能保存经过脱敏且允许返回用户的内容，不能承载原始工具输出。
type Item struct {
	ID         string
	RunID      string
	Sequence   uint32
	Kind       ItemKind
	State      ItemState
	Name       string
	CallID     string
	Summary    string
	ErrorCode  FailureCode
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
}

// Claimed 是后台实例已取得租约、可以执行的一次 Run。
// ActorID 只用于读取所属用户的数据，不会写入事件或返回客户端。
type Claimed struct {
	Run
	ActorID string
}

// StreamEvent 是可通过 SSE 重放的单次 Run 事件。
type StreamEvent struct {
	ID   string
	Type EventType
	Data string
}

// Package execution 管理运维助手一次异步执行的状态、租约和事件规则。
package execution

import "time"

// State 表示一次 Agent 执行的持久状态。
type State string

const (
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
	StateCancelled State = "cancelled"
)

// StepKind 表示一次执行中可持久追踪的步骤类型。
type StepKind string

const (
	StepKindModelCall StepKind = "model_call"
	StepKindToolCall  StepKind = "tool_call"
)

// StepState 表示单个执行步骤的生命周期。
type StepState string

const (
	StepStateRunning   StepState = "running"
	StepStateCompleted StepState = "completed"
	StepStateFailed    StepState = "failed"
	StepStateCancelled StepState = "cancelled"
)

// EventType 是浏览器可订阅和短时重放的流式事件类型。
type EventType string

const (
	EventStarted        EventType = "execution.started"
	EventReasoningDelta EventType = "message.reasoning.delta"
	EventContentDelta   EventType = "message.content.delta"
	EventCompleted      EventType = "execution.completed"
	EventFailed         EventType = "execution.failed"
	EventCancelled      EventType = "execution.cancelled"
	EventStreamFailed   EventType = "stream.failed"
)

// FailureCode 是可持久化并安全返回客户端的稳定错误码。
type FailureCode string

const (
	FailureInternal         FailureCode = "INTERNAL_ERROR"
	FailureModelUnavailable FailureCode = "MODEL_UNAVAILABLE"
	FailureToolUnavailable  FailureCode = "TOOL_UNAVAILABLE"
	FailureEventStore       FailureCode = "EVENT_STORE_UNAVAILABLE"
	FailureWorkerLost       FailureCode = "WORKER_LOST"
	FailureWorkerStopped    FailureCode = "WORKER_STOPPED"
)

// AgentExecution 记录一条用户输入从排队到完成的生命周期。
// 流式分片只短期保存在 Redis，不进入该持久对象。
type AgentExecution struct {
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

// Step 记录一次执行中实际发生的模型或工具调用。
// Summary 只能保存脱敏且允许返回用户的内容，不能承载原始工具输出。
type Step struct {
	ID          string
	ExecutionID string
	Sequence    uint32
	Kind        StepKind
	State       StepState
	Name        string
	CallID      string
	Summary     string
	ErrorCode   FailureCode
	CreatedAt   time.Time
	StartedAt   *time.Time
	FinishedAt  *time.Time
}

// ClaimedExecution 是后台实例已取得租约、可以执行的一次任务。
// ActorID 只用于读取所属用户的数据，不会写入事件或返回客户端。
type ClaimedExecution struct {
	AgentExecution
	ActorID string
}

// StreamEvent 是可通过 SSE 重放的单次执行事件。
type StreamEvent struct {
	ID   string
	Type EventType
	Data string
}

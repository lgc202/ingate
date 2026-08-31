// Package execution 管理运维助手一次异步执行的状态、租约和事件规则。
package execution

import (
	"errors"
	"time"
)

const (
	// StateQueued 表示执行已经创建，正在等待后台实例领取。
	StateQueued State = "queued"
	// StateRunning 表示后台实例持有租约并正在执行任务。
	StateRunning State = "running"
	// StateSucceeded 表示执行已经正常产生最终回答。
	StateSucceeded State = "succeeded"
	// StateFailed 表示执行因不可恢复错误而终止。
	StateFailed State = "failed"
	// StateCancelled 表示执行已响应管理员的取消请求。
	StateCancelled State = "cancelled"
)

const (
	// StepKindModelCall 表示步骤调用了一次模型端点。
	StepKindModelCall StepKind = "model_call"
	// StepKindToolCall 表示步骤调用了一次 Agent 工具。
	StepKindToolCall StepKind = "tool_call"
)

const (
	// StepStateRunning 表示步骤已经开始但尚未结束。
	StepStateRunning StepState = "running"
	// StepStateCompleted 表示步骤已经正常完成。
	StepStateCompleted StepState = "completed"
	// StepStateFailed 表示步骤因不可恢复错误而终止。
	StepStateFailed StepState = "failed"
	// StepStateCancelled 表示步骤随执行取消而终止。
	StepStateCancelled StepState = "cancelled"
)

const (
	// EventStarted 表示执行已经进入运行状态。
	EventStarted EventType = "execution.started"
	// EventReasoningDelta 携带一段可向管理员展示的模型推理内容。
	EventReasoningDelta EventType = "message.reasoning.delta"
	// EventContentDelta 携带一段最终回答内容。
	EventContentDelta EventType = "message.content.delta"
	// EventCompleted 表示执行已经正常完成。
	EventCompleted EventType = "execution.completed"
	// EventFailed 表示执行已经失败。
	EventFailed EventType = "execution.failed"
	// EventCancelled 表示执行已经取消。
	EventCancelled EventType = "execution.cancelled"
	// EventStreamFailed 表示事件流无法继续读取，执行状态需要另行查询。
	EventStreamFailed EventType = "stream.failed"
)

const (
	// FailureInternal 表示未公开内部细节的执行故障。
	FailureInternal FailureCode = "INTERNAL_ERROR"
	// FailureModelUnavailable 表示模型连接当前不可用。
	FailureModelUnavailable FailureCode = "MODEL_UNAVAILABLE"
	// FailureToolUnavailable 表示 Agent 工具依赖当前不可用。
	FailureToolUnavailable FailureCode = "TOOL_UNAVAILABLE"
	// FailureIterationLimit 表示模型未能在限定轮数内完成回答。
	FailureIterationLimit FailureCode = "AGENT_ITERATION_LIMIT"
	// FailureWorkerLost 表示持有任务的后台实例失去租约。
	FailureWorkerLost FailureCode = "WORKER_LOST"
	// FailureWorkerStopped 表示后台实例在执行过程中停止。
	FailureWorkerStopped FailureCode = "WORKER_STOPPED"
)

var (
	// ErrNotFound 表示执行记录对当前管理员不可见。
	ErrNotFound = errors.New("assistant execution not found")
	// ErrStateConflict 表示执行状态已经发生变化，当前操作不能继续。
	ErrStateConflict = errors.New("assistant execution state conflict")
	// ErrConversationBusy 表示同一会话已经存在尚未结束的执行。
	ErrConversationBusy = errors.New("conversation already has an active execution")
	// ErrCancellation 表示执行在当前步骤中观察到取消请求。
	ErrCancellation = errors.New("assistant execution cancellation requested")
	// ErrLeaseLost 表示当前后台实例不再拥有该执行的租约。
	ErrLeaseLost = errors.New("assistant execution lease lost")

	errExecutionRecordUnavailable = errors.New("assistant execution record is unavailable")
)

// State 表示一次 Agent 执行的持久状态。
type State string

// StepKind 表示一次执行中可持久追踪的步骤类型。
type StepKind string

// StepState 表示单个执行步骤的生命周期。
type StepState string

// EventType 是浏览器可订阅和短时重放的流式事件类型。
type EventType string

// FailureCode 是可持久化并安全返回客户端的稳定错误码。
type FailureCode string

// Execution 记录一条用户输入从排队到完成的生命周期。
// 流式分片只短期保存在 Redis，不进入该持久对象。
type Execution struct {
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
	StartedAt   time.Time
	FinishedAt  *time.Time
}

// Claim 是后台实例已取得租约、可以执行的一次任务。
// ActorID 只用于读取所属用户的数据，不会写入事件或返回客户端。
type Claim struct {
	ID             string
	ConversationID string
	ActorID        string
}

// StreamEvent 是可通过 SSE 重放的单次执行事件。
type StreamEvent struct {
	ID   string
	Type EventType
	Data string
}

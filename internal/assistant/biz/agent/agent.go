// Package agent 实现 Ingate 运维 Agent 的模型循环、工具调度和执行事件。
package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"

	agenttool "github.com/lgc202/ingate/internal/assistant/biz/agent/tool"
	"github.com/lgc202/ingate/internal/assistant/biz/modelconfig"
)

const maxIterations = 8

var (
	// ErrModelNotConfigured 表示当前没有可用于运维助手的模型连接。
	ErrModelNotConfigured = errors.New("assistant model is not configured")
	// ErrModelUnavailable 表示模型客户端创建或模型调用失败。
	ErrModelUnavailable = errors.New("assistant model is unavailable")
	// ErrToolUnavailable 表示工具所依赖的 Ingate 内部服务当前不可用。
	ErrToolUnavailable = errors.New("assistant tool is unavailable")
	// ErrIterationLimit 表示模型未能在限定轮数内结束工具调用循环。
	ErrIterationLimit = errors.New("assistant exceeded the model iteration limit")
)

//go:embed prompt/operations.md
var operationsInstruction string

// Role 表示进入模型上下文的消息角色。
// 工具消息由 Eino 在单次循环内维护，不属于跨执行恢复的持久对话。
type Role uint8

const (
	// RoleUser 表示来自管理员的上下文消息。
	RoleUser Role = iota + 1
	// RoleAssistant 表示此前由 Agent 生成的上下文消息。
	RoleAssistant
)

// Message 是 Agent 恢复上下文时需要的最小消息结构。
// ID、会话归属和创建时间属于存储事实，不应进入模型循环协议。
type Message struct {
	Role    Role
	Content string
}

// Request 是一次 Agent 执行的不可变请求。
// 历史消息由执行编排层一次性读取，Agent 循环只在内存中追加模型和工具消息。
type Request struct {
	ExecutionID string
	Messages    []Message
	Resume      *Resume
}

// Resume 指定要恢复的 Eino 中断及管理员提交的结构化决定。
type Resume struct {
	InterruptID string
	Result      *agenttool.ApprovalResult
}

// Response 是 Agent 完成回答或进入审批中断后产生的结果。
// 工具参数和原始工具结果属于循环上下文，不会通过该对象进入持久消息。
type Response struct {
	Content          string
	ReasoningContent string
	Interruption     *ApprovalInterruption
}

// ApprovalInterruption 是当前执行唯一允许暴露给审批流程的根中断。
type ApprovalInterruption struct {
	InterruptID string
	Request     agenttool.ApprovalRequest
}

// ChatModelFactory 将持久化的模型连接转换为单次执行使用的模型客户端。
// 网络协议和厂商 SDK 的差异在 data 层结束，不进入 Agent 循环。
type ChatModelFactory func(
	context.Context,
	modelconfig.Connection,
) (model.ToolCallingChatModel, error)

// CheckpointStore 是 Eino 中断恢复所需的持久化边界。
type CheckpointStore interface {
	Get(context.Context, string) ([]byte, bool, error)
	Set(context.Context, string, []byte) error
}

// Agent 组合运维指令、当前模型连接以及 Ingate 查询和变更提案工具。
// 实例可以并发使用；每次执行的消息、事件和中间状态都保存在方法栈内。
type Agent struct {
	connections     *modelconfig.Usecase
	createChatModel ChatModelFactory
	checkpoints     CheckpointStore
	tools           []einotool.BaseTool
	instruction     string
}

// New 创建运维 Agent，并在进程启动时完成工具定义的静态装配。
func New(
	connections *modelconfig.Usecase,
	source agenttool.QuerySource,
	writer agenttool.ChangeWriter,
	checkpoints CheckpointStore,
	createChatModel ChatModelFactory,
) (*Agent, error) {
	tools, err := agenttool.NewTools(source, writer)
	if err != nil {
		return nil, err
	}
	return &Agent{
		connections:     connections,
		createChatModel: createChatModel,
		checkpoints:     checkpoints,
		tools:           tools,
		instruction:     strings.TrimSpace(operationsInstruction),
	}, nil
}

// Execute 完成一次独立的 Agent 循环，并通过结构化事件暴露执行过程。
func (a *Agent) Execute(
	ctx context.Context,
	request Request,
	events EventSink,
) (Response, error) {
	connection, err := a.connections.ActiveConnection(ctx)
	if errors.Is(err, modelconfig.ErrNotConfigured) {
		return Response{}, ErrModelNotConfigured
	}
	if err != nil {
		return Response{}, fmt.Errorf("load assistant model connection: %w", err)
	}

	// 先发布选模事实，再创建远端客户端。这样即使厂商连接失败，执行详情也能说明
	// 本次实际选择了哪个模型，而不是留下一个无法解释的空步骤。
	if err := events.Emit(ctx, ModelSelected{Model: connection.Model}); err != nil {
		return Response{}, fmt.Errorf("record selected assistant model: %w", err)
	}
	chatModel, err := a.createChatModel(ctx, connection)
	if err != nil {
		return Response{}, fmt.Errorf(
			"%w: create assistant chat model: %w",
			ErrModelUnavailable,
			err,
		)
	}

	// Eino 只负责当前模型—工具循环。模型选择、任务领取和持久状态仍由
	// Ingate 业务层决定，不把服务编排职责藏入 Agent 框架。
	return a.runModelLoop(
		ctx,
		request,
		events,
		connection.Model,
		chatModel,
	)
}

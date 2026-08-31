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

//go:embed prompt/operations.md
var operationsInstruction string

// ChatModelFactory 将持久化的模型连接转换为单次执行使用的模型客户端。
// 网络协议和厂商 SDK 的差异在 data 层结束，不进入 Agent 循环。
type ChatModelFactory func(
	context.Context,
	modelconfig.Connection,
) (model.ToolCallingChatModel, error)

// Agent 组合运维指令、当前模型连接和 Ingate 只读工具。
// 实例可以并发使用；每次执行的消息、事件和中间状态都保存在方法栈内。
type Agent struct {
	connections     *modelconfig.Usecase
	createChatModel ChatModelFactory
	tools           []einotool.BaseTool
	instruction     string
}

// New 创建运维 Agent，并在进程启动时完成工具定义的静态装配。
func New(
	connections *modelconfig.Usecase,
	source agenttool.QuerySource,
	createChatModel ChatModelFactory,
) (*Agent, error) {
	tools, err := agenttool.NewTools(source)
	if err != nil {
		return nil, err
	}
	return &Agent{
		connections:     connections,
		createChatModel: createChatModel,
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
		return Response{}, err
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

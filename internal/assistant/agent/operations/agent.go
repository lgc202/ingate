// Package operations 实现面向 Ingate 配置与流量排障的运维 Agent。
package operations

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"

	agentprotocol "github.com/lgc202/ingate/internal/assistant/agent"
	agenttool "github.com/lgc202/ingate/internal/assistant/agent/tool"
	modelbiz "github.com/lgc202/ingate/internal/assistant/biz/model"
)

const maxIterations = 8

// ChatModelFactory 将持久化的模型连接转换为单次执行使用的模型客户端。
// 网络协议和厂商 SDK 的差异在 data 层结束，不进入 Agent 循环。
type ChatModelFactory func(
	context.Context,
	modelbiz.Connection,
) (model.ToolCallingChatModel, error)

// Agent 组合运维指令、当前模型连接和 Ingate 只读工具。
// 实例可以并发使用；每次执行的消息、事件和中间状态都保存在方法栈内。
type Agent struct {
	connections     *modelbiz.Service
	createChatModel ChatModelFactory
	tools           []einotool.BaseTool
	instruction     string
}

// New 创建运维 Agent，并在进程启动时完成工具定义的静态装配。
func New(
	connections *modelbiz.Service,
	source agenttool.OperationsSource,
	createChatModel ChatModelFactory,
) (*Agent, error) {
	tools, err := agenttool.NewOperations(source)
	if err != nil {
		return nil, err
	}
	return &Agent{
		connections:     connections,
		createChatModel: createChatModel,
		tools:           tools,
		instruction:     systemInstruction(),
	}, nil
}

// Execute 完成一次独立的 Agent 循环，并通过结构化事件暴露执行过程。
func (a *Agent) Execute(
	ctx context.Context,
	request agentprotocol.Request,
	events agentprotocol.EventSink,
) (agentprotocol.Response, error) {
	connection, err := a.connections.ActiveConnection(ctx)
	if errors.Is(err, modelbiz.ErrNotConfigured) {
		return agentprotocol.Response{}, agentprotocol.ErrModelNotConfigured
	}
	if err != nil {
		return agentprotocol.Response{}, fmt.Errorf("load assistant model connection: %w", err)
	}

	// 先发布选模事实，再创建远端客户端。这样即使厂商连接失败，执行详情也能说明
	// 本次实际选择了哪个模型，而不是留下一个无法解释的空步骤。
	if err := events.Emit(ctx, agentprotocol.ModelSelected{Model: connection.Model}); err != nil {
		return agentprotocol.Response{}, fmt.Errorf("record selected assistant model: %w", err)
	}
	chatModel, err := a.createChatModel(ctx, connection)
	if err != nil {
		return agentprotocol.Response{}, err
	}

	// Eino 只负责单次模型循环。模型选择和连接配置仍由 Ingate 业务层决定，
	// 因而未来替换 Agent 框架不会影响任务领取、持久状态和浏览器事件协议。
	loop := newModelLoop(connection.Model, chatModel, a.instruction, a.tools)
	return loop.Execute(ctx, request, events)
}

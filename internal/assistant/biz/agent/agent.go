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

const baseInstruction = `你是 Ingate 运维助手。
涉及当前系统状态、配置或资源关系时，必须先调用只读工具核实，不能根据用户描述猜测。
比较请求量、错误率或延迟时，优先使用 analyze_traffic 一次完成分组和排序；只有需要配置细节时才查询资源列表。
排查具体失败请求时，先用 analyze_traffic 定位资源，再把排名中的 resource_id 和相同时间范围交给 list_recent_failures；不要为了查名称逐条调用资源列表。
发现某条路由异常后，使用 get_route_configuration 一次核对路由、关联网关和目标服务；不要分别枚举三类资源来拼接关系。
当前工具只提供查询能力，不能声称已经修改系统。需要变更时说明方案和影响，并等待用户确认。`

//go:embed prompt/gateway-configuration-diagnosis.md
var gatewayConfigurationDiagnosis string

// ChatModelFactory 将持久化的模型连接转换为单次执行使用的模型客户端。
// 网络协议和厂商 SDK 的差异在 data 层结束，不进入 Agent 循环。
type ChatModelFactory func(
	context.Context,
	modelconfig.Connection,
) (model.ToolCallingChatModel, error)

// Agent 组合运维指令、当前模型连接和 Ingate 只读工具。
// 实例可以并发使用；每次执行的消息、事件和中间状态都保存在方法栈内。
type Agent struct {
	connections     *modelconfig.Service
	createChatModel ChatModelFactory
	tools           []einotool.BaseTool
	instruction     string
}

// New 创建运维 Agent，并在进程启动时完成工具定义的静态装配。
func New(
	connections *modelconfig.Service,
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
		instruction:     baseInstruction + "\n\n" + strings.TrimSpace(gatewayConfigurationDiagnosis),
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
	return executeModelLoop(
		ctx,
		request,
		events,
		connection.Model,
		chatModel,
		a.instruction,
		a.tools,
	)
}

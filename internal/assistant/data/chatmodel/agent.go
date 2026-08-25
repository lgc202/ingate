// Package chatmodel 通过 Eino ADK 执行运维助手模型。
package chatmodel

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	modelbiz "github.com/lgc202/ingate/internal/assistant/biz/model"
	runbiz "github.com/lgc202/ingate/internal/assistant/biz/run"
	"github.com/lgc202/ingate/internal/assistant/data/adminapi"
	"github.com/lgc202/ingate/internal/assistant/data/chatmodel/skill"
	agenttool "github.com/lgc202/ingate/internal/assistant/data/chatmodel/tool"
)

const (
	maxIterations = 8
	instruction   = `你是 Ingate 运维助手。
涉及当前系统状态、配置或资源关系时，必须先调用只读工具核实，不能根据用户描述猜测。
当前工具只提供查询能力，不能声称已经修改系统。需要变更时说明方案和影响，并等待用户确认。`
)

// Agent 在每次 Run 开始时读取当前模型连接。
// 这样新配置无需重启进程，已开始的 Run 仍使用自己的配置快照。
type Agent struct {
	connections *modelbiz.Service
	tools       *agenttool.Registry
	skills      *skill.Catalog
}

// NewAgent 创建模型选取器和进程内共享的只读工具定义。
func NewAgent(connections *modelbiz.Service, client *adminapi.Client) (*Agent, error) {
	tools, err := agenttool.NewRegistry(client)
	if err != nil {
		return nil, err
	}
	skills, err := skill.LoadBuiltin()
	if err != nil {
		return nil, err
	}
	for _, definition := range skills.Definitions() {
		if err := tools.Validate(definition.AllowedTools); err != nil {
			return nil, fmt.Errorf("validate assistant skill %q: %w", definition.Name, err)
		}
	}
	return &Agent{
		connections: connections,
		tools:       tools,
		skills:      skills,
	}, nil
}

// Execute 读取本次 Run 的模型连接，并用 Eino 执行工具调用循环。
// SelectModel 成功后才开始模型请求，确保业务层先记录实际模型并发布 started 事件。
func (a *Agent) Execute(ctx context.Context, request runbiz.AgentRequest) (runbiz.Result, error) {
	connection, err := a.connections.ForRun(ctx)
	if errors.Is(err, modelbiz.ErrNotConfigured) {
		return runbiz.Result{}, runbiz.ErrModelNotConfigured
	}
	if err != nil {
		return runbiz.Result{}, fmt.Errorf("load assistant model connection: %w", err)
	}
	chatModel, err := newChatModel(ctx, connection)
	if err != nil {
		return runbiz.Result{}, err
	}
	if err := request.SelectModel(ctx, connection.Model); err != nil {
		return runbiz.Result{}, fmt.Errorf("select assistant model: %w", err)
	}
	return a.generate(ctx, connection.Model, chatModel, request)
}

func (a *Agent) generate(
	ctx context.Context,
	modelName string,
	chatModel model.ToolCallingChatModel,
	request runbiz.AgentRequest,
) (runbiz.Result, error) {
	skillSession := skill.NewSession(a.skills)
	loadSkill, err := skill.NewLoadTool(skillSession)
	if err != nil {
		return runbiz.Result{}, err
	}
	registeredTools := a.tools.All()
	tools := make([]tool.BaseTool, 0, len(registeredTools)+1)
	tools = append(tools, loadSkill)
	tools = append(tools, registeredTools...)

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "ingate_operations_assistant",
		Description: "Helps operators understand and operate the current Ingate environment",
		Instruction: a.skills.Instruction(instruction),
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools:               tools,
			ExecuteSequentially: true,
		}},
		MaxIterations: maxIterations,
		Middlewares: []adk.AgentMiddleware{newExecutionMiddleware(
			modelName,
			request.Recorder,
			func(name string) error {
				if name == skill.LoadToolName {
					return nil
				}
				return skillSession.Authorize(name)
			},
		)},
	})
	if err != nil {
		return runbiz.Result{}, fmt.Errorf("create Eino chat model agent: %w", err)
	}
	messages := make([]adk.Message, 0, len(request.Messages))
	for _, message := range request.Messages {
		switch message.Role {
		case conversation.RoleUser:
			messages = append(messages, schema.UserMessage(message.Content))
		case conversation.RoleAssistant:
			messages = append(messages, schema.AssistantMessage(message.Content, nil))
		}
	}
	reply := replyBuilder{emit: request.Emit}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true})
	iterator := runner.Run(ctx, messages)
	for event, ok := iterator.Next(); ok; event, ok = iterator.Next() {
		if event.Err != nil {
			return runbiz.Result{}, fmt.Errorf("run Eino agent: %w", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		output := event.Output.MessageOutput
		if !output.IsStreaming {
			if err := reply.append(output.Message); err != nil {
				return runbiz.Result{}, err
			}
			continue
		}
		stream := output.MessageStream
		for {
			chunk, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				stream.Close()
				return runbiz.Result{}, fmt.Errorf("read Eino model stream: %w", err)
			}
			if err := reply.append(chunk); err != nil {
				stream.Close()
				return runbiz.Result{}, err
			}
		}
		stream.Close()
	}
	return reply.result(), nil
}

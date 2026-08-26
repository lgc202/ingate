// Package agent 通过 Eino ADK 组合模型、专业指令和工具。
package agent

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	agenttool "github.com/lgc202/ingate/internal/assistant/agent/tool"
	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	executionbiz "github.com/lgc202/ingate/internal/assistant/biz/execution"
	modelbiz "github.com/lgc202/ingate/internal/assistant/biz/model"
)

const maxIterations = 8

// ChatModelFactory 将持久化的模型连接转换为本次执行使用的模型客户端。
// Agent 只依赖这个能力，不感知具体厂商 SDK 和网络协议实现。
type ChatModelFactory func(
	ctx context.Context,
	connection modelbiz.Connection,
) (model.ToolCallingChatModel, error)

// Agent 在每次执行开始时读取当前模型连接。
// 这样新配置无需重启进程，已开始的执行仍使用自己的连接信息。
type Agent struct {
	connections     *modelbiz.Service
	createChatModel ChatModelFactory
	tools           *agenttool.Registry
	instruction     string
}

// NewAgent 创建模型选取器，并装配进程内共享的只读工具与模型创建能力。
func NewAgent(
	connections *modelbiz.Service,
	resources agenttool.ResourceReader,
	createChatModel ChatModelFactory,
) (*Agent, error) {
	tools, err := agenttool.NewRegistry(resources)
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

// Execute 读取本次执行的模型连接，并用 Eino 执行工具调用循环。
// SelectModel 成功后才开始模型请求，确保业务层先记录实际模型并发布 started 事件。
func (a *Agent) Execute(
	ctx context.Context,
	request executionbiz.AgentRequest,
) (executionbiz.AgentResult, error) {
	connection, err := a.connections.ActiveConnection(ctx)
	if errors.Is(err, modelbiz.ErrNotConfigured) {
		return executionbiz.AgentResult{}, executionbiz.ErrModelNotConfigured
	}
	if err != nil {
		return executionbiz.AgentResult{}, fmt.Errorf("load assistant model connection: %w", err)
	}
	chatModel, err := a.createChatModel(ctx, connection)
	if err != nil {
		return executionbiz.AgentResult{}, err
	}
	if err := request.SelectModel(ctx, connection.Model); err != nil {
		return executionbiz.AgentResult{}, fmt.Errorf("select assistant model: %w", err)
	}
	return a.generate(ctx, connection.Model, chatModel, request)
}

func (a *Agent) generate(
	ctx context.Context,
	modelName string,
	chatModel model.ToolCallingChatModel,
	request executionbiz.AgentRequest,
) (executionbiz.AgentResult, error) {
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "ingate_operations_assistant",
		Description: "Helps operators understand and operate the current Ingate environment",
		Instruction: a.instruction,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools:               a.tools.All(),
			ExecuteSequentially: true,
		}},
		MaxIterations: maxIterations,
		Middlewares:   []adk.AgentMiddleware{newExecutionMiddleware(modelName, request.Recorder)},
	})
	if err != nil {
		return executionbiz.AgentResult{}, fmt.Errorf("create Eino chat model agent: %w", err)
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
	agentRunner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true})
	iterator := agentRunner.Run(ctx, messages)
	for event, ok := iterator.Next(); ok; event, ok = iterator.Next() {
		if event.Err != nil {
			return executionbiz.AgentResult{}, fmt.Errorf("execute Eino agent: %w", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		output := event.Output.MessageOutput
		if !output.IsStreaming {
			if err := reply.append(output.Message); err != nil {
				return executionbiz.AgentResult{}, err
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
				return executionbiz.AgentResult{}, fmt.Errorf("read Eino model stream: %w", err)
			}
			if err := reply.append(chunk); err != nil {
				stream.Close()
				return executionbiz.AgentResult{}, err
			}
		}
		stream.Close()
	}
	return reply.result(), nil
}

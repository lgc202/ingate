package operations

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	agentprotocol "github.com/lgc202/ingate/internal/assistant/agent"
)

// modelLoop 封装一次 Eino Agent 循环所需的静态依赖。
// 它不负责选择模型、保存消息或更新任务状态，只消费已经确定的请求并返回响应。
type modelLoop struct {
	modelName   string
	chatModel   model.ToolCallingChatModel
	instruction string
	tools       []einotool.BaseTool
}

func newModelLoop(
	modelName string,
	chatModel model.ToolCallingChatModel,
	instruction string,
	tools []einotool.BaseTool,
) modelLoop {
	// Eino 会把工具定义交给本次 Agent。复制切片可以避免框架代码持有并修改
	// 进程级 Agent 的工具集合；工具实例本身按官方约定可并发调用。
	definitions := make([]einotool.BaseTool, len(tools))
	copy(definitions, tools)
	return modelLoop{
		modelName:   modelName,
		chatModel:   chatModel,
		instruction: instruction,
		tools:       definitions,
	}
}

// Execute 建立并消费一次模型—工具循环。
func (l modelLoop) Execute(
	ctx context.Context,
	request agentprotocol.Request,
	events agentprotocol.EventSink,
) (agentprotocol.Response, error) {
	chatAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "ingate_operations_assistant",
		Description: "Helps operators understand and operate the current Ingate environment",
		Instruction: l.instruction,
		Model:       l.chatModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools:               l.tools,
			ExecuteSequentially: true,
		}},
		MaxIterations: maxIterations,
		Middlewares:   []adk.AgentMiddleware{newEventMiddleware(l.modelName, events)},
	})
	if err != nil {
		return agentprotocol.Response{}, fmt.Errorf("create Eino chat model agent: %w", err)
	}

	messages := modelMessages(request.Messages)
	response := responseBuilder{events: events}
	iterator := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           chatAgent,
		EnableStreaming: true,
	}).Run(ctx, messages)
	for event, ok := iterator.Next(); ok; event, ok = iterator.Next() {
		if event.Err != nil {
			return agentprotocol.Response{}, fmt.Errorf("execute Eino agent: %w", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		if err := response.appendOutput(ctx, event.Output.MessageOutput); err != nil {
			return agentprotocol.Response{}, err
		}
	}
	return response.build(), nil
}

func modelMessages(messages []agentprotocol.Message) []adk.Message {
	// 数据库消息在进入循环时一次性转换。之后的模型消息和工具结果由 Eino 在本次
	// 内存上下文中维护，不反向污染会话的持久消息。
	result := make([]adk.Message, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case agentprotocol.RoleUser:
			result = append(result, schema.UserMessage(message.Content))
		case agentprotocol.RoleAssistant:
			result = append(result, schema.AssistantMessage(message.Content, nil))
		}
	}
	return result
}

func (b *responseBuilder) appendOutput(
	ctx context.Context,
	output *adk.MessageVariant,
) error {
	if !output.IsStreaming {
		return b.append(ctx, output.Message)
	}

	defer output.MessageStream.Close()
	for {
		chunk, err := output.MessageStream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read Eino model stream: %w", err)
		}
		if err := b.append(ctx, chunk); err != nil {
			return err
		}
	}
}

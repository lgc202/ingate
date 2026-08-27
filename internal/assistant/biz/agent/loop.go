package agent

import (
	"context"
	"fmt"
	"slices"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// executeModelLoop 建立并消费一次 Eino 模型—工具循环。
// 调用方已经完成模型选择和上下文恢复，本函数只负责把明确的输入交给 Eino。
func executeModelLoop(
	ctx context.Context,
	request Request,
	events EventSink,
	modelName string,
	chatModel model.ToolCallingChatModel,
	instruction string,
	tools []einotool.BaseTool,
) (Response, error) {
	// Eino 会把工具定义交给本次 Agent。复制切片可以避免框架代码持有并修改
	// 进程级 Agent 的工具集合；工具实例本身按官方约定可并发调用。
	chatAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "ingate_operations_assistant",
		Description: "Helps operators understand and operate the current Ingate environment",
		Instruction: instruction,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				// 当前工具都是只读查询，沿用 Eino 默认的并行调度。模型在同一轮
				// 比较多个资源时不应被框架外的人为串行化。
				Tools: slices.Clone(tools),
			},
		},
		MaxIterations: maxIterations,
		Middlewares:   []adk.AgentMiddleware{newEventMiddleware(modelName, events)},
	})
	if err != nil {
		return Response{}, fmt.Errorf("create Eino chat model agent: %w", err)
	}

	messages := modelMessages(request.Messages)
	response := responseCollector{events: events}
	iterator := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           chatAgent,
		EnableStreaming: true,
	}).Run(ctx, messages)
	for event, ok := iterator.Next(); ok; event, ok = iterator.Next() {
		if err := response.consume(ctx, event); err != nil {
			return Response{}, err
		}
	}
	return response.build(), nil
}

func modelMessages(messages []Message) []adk.Message {
	// 数据库消息在进入循环时一次性转换。之后的模型消息和工具结果由 Eino 在本次
	// 内存上下文中维护，不反向污染会话的持久消息。
	result := make([]adk.Message, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case RoleUser:
			result = append(result, schema.UserMessage(message.Content))
		case RoleAssistant:
			result = append(result, schema.AssistantMessage(message.Content, nil))
		}
	}
	return result
}

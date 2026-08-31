package agent

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// runModelLoop 建立并消费一次 Eino 模型—工具循环。
// 首次输入和审批恢复都属于同一次持久化 Execution。
func (a *Agent) runModelLoop(
	ctx context.Context,
	request Request,
	events EventSink,
	modelName string,
	chatModel model.ToolCallingChatModel,
) (Response, error) {
	// Eino 会把工具定义交给本次 Agent。复制切片可以避免框架代码持有并修改
	// 进程级 Agent 的工具集合；工具实例本身按官方约定可并发调用。
	chatAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "ingate_operations_assistant",
		Description: "Inspects Ingate and performs explicitly approved configuration changes",
		Instruction: a.instruction,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				// 一次运维请求只允许一个配置写入工具。工具本身通过 Eino interrupt
				// 暂停，收到明确批准后才在恢复路径执行写入。
				Tools: slices.Clone(a.tools),
			},
		},
		MaxIterations: maxIterations,
		Handlers:      []adk.ChatModelAgentMiddleware{newEventHandler(modelName, events)},
	})
	if err != nil {
		return Response{}, fmt.Errorf("create Eino chat model agent: %w", err)
	}

	if strings.TrimSpace(request.ExecutionID) == "" {
		return Response{}, errors.New("agent execution ID is empty")
	}
	response := responseCollector{events: events}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           chatAgent,
		EnableStreaming: true,
		CheckPointStore: a.checkpoints,
	})
	var iterator *adk.AsyncIterator[*adk.AgentEvent]
	if request.Resume == nil {
		messages := modelMessages(request.Messages)
		if len(messages) == 0 {
			return Response{}, errors.New("agent execution contains no messages")
		}
		iterator = runner.Run(ctx, messages, adk.WithCheckPointID(request.ExecutionID))
	} else {
		if strings.TrimSpace(request.Resume.InterruptID) == "" || request.Resume.Result == nil {
			return Response{}, errors.New("agent resume request is incomplete")
		}
		iterator, err = runner.ResumeWithParams(
			ctx,
			request.ExecutionID,
			&adk.ResumeParams{Targets: map[string]any{
				request.Resume.InterruptID: request.Resume.Result,
			}},
		)
		if err != nil {
			return Response{}, fmt.Errorf("resume Eino agent: %w", err)
		}
	}
	for event, ok := iterator.Next(); ok; event, ok = iterator.Next() {
		if err := response.consume(ctx, event); err != nil {
			return Response{}, err
		}
	}
	result := response.build()
	if result.Interruption == nil && strings.TrimSpace(result.Content) == "" {
		return Response{}, errors.New("agent completed without an assistant response")
	}
	return result, nil
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

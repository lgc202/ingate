package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"

	agentprotocol "github.com/lgc202/ingate/internal/assistant/agent"
	agenttool "github.com/lgc202/ingate/internal/assistant/agent/tool"
)

// eventMiddleware 把 Eino 内部回调转换成稳定的 Agent 事件。
// 它按执行创建，modelCallID 不会在并发会话之间共享。
type eventMiddleware struct {
	modelName   string
	events      agentprotocol.EventSink
	modelCallID string
}

type invalidToolInputOutput struct {
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

func newEventMiddleware(modelName string, events agentprotocol.EventSink) adk.AgentMiddleware {
	middleware := &eventMiddleware{modelName: modelName, events: events}
	return adk.AgentMiddleware{
		BeforeChatModel: middleware.beforeChatModel,
		AfterChatModel:  middleware.afterChatModel,
		WrapToolCall: compose.ToolMiddleware{
			Invokable: middleware.wrapToolCall,
		},
	}
}

func (m *eventMiddleware) beforeChatModel(
	ctx context.Context,
	_ *adk.ChatModelAgentState,
) error {
	callID := uuid.NewString()
	if err := m.events.Emit(ctx, agentprotocol.ModelCallStarted{
		CallID: callID,
		Model:  m.modelName,
	}); err != nil {
		return err
	}
	m.modelCallID = callID
	return nil
}

func (m *eventMiddleware) afterChatModel(
	ctx context.Context,
	state *adk.ChatModelAgentState,
) error {
	message := state.Messages[len(state.Messages)-1]
	summary := "模型已生成回答"
	if len(message.ToolCalls) > 0 {
		summary = fmt.Sprintf("模型选择了 %d 个工具", len(message.ToolCalls))
	}
	if err := m.events.Emit(ctx, agentprotocol.ModelCallCompleted{
		CallID:  m.modelCallID,
		Model:   m.modelName,
		Summary: summary,
	}); err != nil {
		return err
	}
	m.modelCallID = ""
	return nil
}

func (m *eventMiddleware) wrapToolCall(
	next compose.InvokableToolEndpoint,
) compose.InvokableToolEndpoint {
	return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		callID := input.CallID
		if callID == "" {
			// 部分兼容模型不返回工具调用 ID；内部生成关联值后，步骤仍能准确结束。
			callID = uuid.NewString()
		}
		if err := m.events.Emit(ctx, agentprotocol.ToolCallStarted{
			CallID: callID,
			Tool:   input.Name,
		}); err != nil {
			return nil, err
		}

		output, err := next(ctx, input)
		if err != nil {
			if reason, ok := agenttool.InvalidInputReason(err); ok {
				// 模型生成的参数不完整不是系统故障。把可修正原因作为工具结果送回循环，
				// 让模型自行补全参数；此步骤在观测上仍是一次正常完成的工具调用。
				result, marshalErr := json.Marshal(invalidToolInputOutput{
					Summary: reason,
					Status:  "invalid_input",
				})
				if marshalErr != nil {
					return nil, m.failTool(ctx, callID, input.Name, marshalErr)
				}
				if eventErr := m.events.Emit(ctx, agentprotocol.ToolCallCompleted{
					CallID:  callID,
					Tool:    input.Name,
					Summary: "工具参数不完整，已请求模型补全后重试",
				}); eventErr != nil {
					return nil, eventErr
				}
				return &compose.ToolOutput{Result: string(result)}, nil
			}
			return nil, m.failTool(ctx, callID, input.Name, err)
		}

		summary, err := toolResultSummary(output.Result)
		if err != nil {
			return nil, m.failTool(ctx, callID, input.Name, err)
		}
		if err := m.events.Emit(ctx, agentprotocol.ToolCallCompleted{
			CallID:  callID,
			Tool:    input.Name,
			Summary: summary,
		}); err != nil {
			return nil, err
		}
		return output, nil
	}
}

func (m *eventMiddleware) failTool(
	ctx context.Context,
	callID string,
	name string,
	cause error,
) error {
	eventErr := m.events.Emit(ctx, agentprotocol.ToolCallFailed{
		CallID: callID,
		Tool:   name,
	})
	return errors.Join(
		agentprotocol.ErrToolUnavailable,
		fmt.Errorf("execute assistant tool %q: %w", name, cause),
		eventErr,
	)
}

func toolResultSummary(result string) (string, error) {
	var output struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		return "", fmt.Errorf("decode assistant tool result: %w", err)
	}
	output.Summary = strings.TrimSpace(output.Summary)
	if output.Summary == "" {
		return "", errors.New("assistant tool result summary is empty")
	}
	return output.Summary, nil
}

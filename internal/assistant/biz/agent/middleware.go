package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"
)

// eventMiddleware 把 Eino 内部回调转换成稳定的 Agent 事件。
// Middleware 只观察调用生命周期，不修正工具参数，也不改写工具执行结果。
// 它按执行创建，modelCallID 不会在并发会话之间共享。
type eventMiddleware struct {
	modelName   string
	events      EventSink
	modelCallID string
}

func newEventMiddleware(modelName string, events EventSink) adk.AgentMiddleware {
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
	if m.modelCallID != "" {
		return errors.New("agent started a model call before completing the previous call")
	}
	callID := uuid.NewString()
	if err := m.events.Emit(ctx, ModelCallStarted{
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
	if m.modelCallID == "" {
		return errors.New("agent completed a model call without a matching start event")
	}
	if state == nil || len(state.Messages) == 0 {
		return errors.New("agent completed a model call without a response message")
	}
	message := state.Messages[len(state.Messages)-1]
	if message == nil {
		return errors.New("agent completed a model call with a nil response message")
	}
	summary := "模型已生成回答"
	if len(message.ToolCalls) > 0 {
		summary = fmt.Sprintf("模型选择了 %d 个工具", len(message.ToolCalls))
	}
	if err := m.events.Emit(ctx, ModelCallCompleted{
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
		if input == nil {
			return nil, errors.New("agent invoked a tool with nil input")
		}
		callID := input.CallID
		if callID == "" {
			// 部分兼容模型不返回工具调用 ID；内部生成关联值后，步骤仍能准确结束。
			callID = uuid.NewString()
		}
		if err := m.events.Emit(ctx, ToolCallStarted{
			CallID: callID,
			Tool:   input.Name,
		}); err != nil {
			return nil, err
		}

		output, err := next(ctx, input)
		if err != nil {
			return nil, m.failTool(ctx, callID, input.Name, err)
		}
		if output == nil {
			return nil, m.failTool(
				ctx,
				callID,
				input.Name,
				errors.New("tool returned nil output"),
			)
		}

		summary, err := toolResultSummary(output.Result)
		if err != nil {
			return nil, m.failTool(ctx, callID, input.Name, err)
		}
		if err := m.events.Emit(ctx, ToolCallCompleted{
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
	toolErr := fmt.Errorf("execute assistant tool %q: %w", name, cause)
	eventErr := m.events.Emit(ctx, ToolCallFailed{
		CallID: callID,
		Tool:   name,
	})
	if eventErr != nil {
		// 执行记录写入失败时，以持久化故障作为最终分类。若仍附带工具不可用标记，
		// 上层会把数据库故障误报成 Admin API 或分析服务不可用。
		return errors.Join(toolErr, eventErr)
	}
	return errors.Join(ErrToolUnavailable, toolErr)
}

func toolResultSummary(result string) (string, error) {
	var output struct {
		Summary string `json:"summary"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		return "", fmt.Errorf("decode assistant tool result: %w", err)
	}
	output.Summary = strings.TrimSpace(output.Summary)
	if output.Summary == "" {
		return "", errors.New("assistant tool result summary is empty")
	}
	switch output.Status {
	case "complete", "partial":
		return output.Summary, nil
	case "invalid_input":
		// 执行详情只展示稳定事实；具体参数修正原因仅保留在 Eino 循环内。
		return "工具参数无效，已将修正原因返回模型", nil
	case "not_found":
		return "工具查询目标已不存在，已返回模型重新定位", nil
	default:
		return "", fmt.Errorf("assistant tool result has unsupported status %q", output.Status)
	}
}

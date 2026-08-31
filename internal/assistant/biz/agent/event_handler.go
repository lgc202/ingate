package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/google/uuid"

	agenttool "github.com/lgc202/ingate/internal/assistant/biz/agent/tool"
	changebiz "github.com/lgc202/ingate/internal/assistant/biz/change"
)

// eventHandler 把 Eino 调用生命周期转换成 Ingate 的稳定执行事件。
// Handler 只观察调用，不修改模型消息、工具参数或工具结果。
type eventHandler struct {
	*adk.BaseChatModelAgentMiddleware

	modelName   string
	events      EventSink
	modelCallID string
}

type parsedToolResult struct {
	Summary    string                `json:"summary"`
	Status     string                `json:"status"`
	ChangeID   string                `json:"change_id"`
	ResourceID string                `json:"resource_id"`
	ErrorCode  changebiz.FailureCode `json:"error_code"`
}

func (h *eventHandler) BeforeModelRewriteState(
	ctx context.Context,
	state *adk.ChatModelAgentState,
	_ *adk.ModelContext,
) (context.Context, *adk.ChatModelAgentState, error) {
	if h.modelCallID != "" {
		return ctx, state, errors.New("agent started a model call before completing the previous call")
	}

	callID := uuid.NewString()
	if err := h.events.Emit(ctx, ModelCallStarted{
		CallID: callID,
		Model:  h.modelName,
	}); err != nil {
		return ctx, state, err
	}
	h.modelCallID = callID
	return ctx, state, nil
}

func (h *eventHandler) AfterModelRewriteState(
	ctx context.Context,
	state *adk.ChatModelAgentState,
	_ *adk.ModelContext,
) (context.Context, *adk.ChatModelAgentState, error) {
	if h.modelCallID == "" {
		return ctx, state, errors.New("agent completed a model call without a matching start event")
	}
	if state == nil || len(state.Messages) == 0 {
		return ctx, state, errors.New("agent completed a model call without a response message")
	}
	message := state.Messages[len(state.Messages)-1]
	if message == nil {
		return ctx, state, errors.New("agent completed a model call with a nil response message")
	}

	summary := "模型已生成回答"
	if len(message.ToolCalls) > 0 {
		summary = fmt.Sprintf("模型选择了 %d 个工具", len(message.ToolCalls))
	}
	if err := h.events.Emit(ctx, ModelCallCompleted{
		CallID:  h.modelCallID,
		Model:   h.modelName,
		Summary: summary,
	}); err != nil {
		return ctx, state, err
	}
	h.modelCallID = ""
	return ctx, state, nil
}

func (h *eventHandler) WrapInvokableToolCall(
	_ context.Context,
	next adk.InvokableToolCallEndpoint,
	toolContext *adk.ToolContext,
) (adk.InvokableToolCallEndpoint, error) {
	if toolContext == nil {
		return nil, errors.New("agent prepared a tool call without context")
	}

	return func(
		ctx context.Context,
		arguments string,
		options ...einotool.Option,
	) (string, error) {
		callID := toolContext.CallID
		if callID == "" {
			// 部分兼容模型不返回工具调用 ID；内部生成关联值后，步骤仍能准确结束。
			callID = uuid.NewString()
		}
		wasInterrupted, _, _ := einotool.GetInterruptState[any](ctx)
		if !wasInterrupted {
			if err := h.events.Emit(ctx, ToolCallStarted{
				CallID: callID,
				Tool:   toolContext.Name,
			}); err != nil {
				return "", err
			}
		}

		result, err := next(ctx, arguments, options...)
		if err != nil {
			if agenttool.IsApprovalInterrupt(err) {
				return "", err
			}
			return "", h.failTool(ctx, callID, toolContext.Name, err)
		}
		output, err := parseToolResult(result)
		if err != nil {
			return "", h.failTool(ctx, callID, toolContext.Name, err)
		}
		if output.ChangeID != "" && output.Status != "change_rejected" {
			if err := h.events.Emit(ctx, ChangeCompleted{
				ID:         output.ChangeID,
				State:      changeState(output.Status),
				ResourceID: output.ResourceID,
				ErrorCode:  output.ErrorCode,
			}); err != nil {
				return "", err
			}
		}
		if err := h.events.Emit(ctx, ToolCallCompleted{
			CallID:  callID,
			Tool:    toolContext.Name,
			Summary: output.Summary,
		}); err != nil {
			return "", err
		}
		return result, nil
	}, nil
}

func newEventHandler(modelName string, events EventSink) *eventHandler {
	return &eventHandler{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
		modelName:                    modelName,
		events:                       events,
	}
}

func (h *eventHandler) failTool(
	ctx context.Context,
	callID string,
	name string,
	cause error,
) error {
	toolErr := fmt.Errorf("execute assistant tool %q: %w", name, cause)
	eventErr := h.events.Emit(ctx, ToolCallFailed{
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

func parseToolResult(result string) (parsedToolResult, error) {
	var output parsedToolResult
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		return parsedToolResult{}, fmt.Errorf("decode assistant tool result: %w", err)
	}
	output.Summary = strings.TrimSpace(output.Summary)
	if output.Summary == "" {
		return parsedToolResult{}, errors.New("assistant tool result summary is empty")
	}
	switch output.Status {
	case "complete", "partial":
		return output, nil
	case "change_rejected":
		if !validChangeResultID(output.ChangeID) || output.ResourceID != "" || output.ErrorCode != "" {
			return parsedToolResult{}, errors.New("rejected change tool result is invalid")
		}
		return output, nil
	case "change_succeeded":
		if !validChangeResultID(output.ChangeID) || !validChangeResultID(output.ResourceID) ||
			output.ErrorCode != "" {
			return parsedToolResult{}, errors.New("successful change tool result is invalid")
		}
		return output, nil
	case "change_failed":
		if !validChangeResultID(output.ChangeID) || output.ResourceID != "" ||
			output.ErrorCode != changebiz.FailureAdminRejected {
			return parsedToolResult{}, errors.New("failed change tool result is invalid")
		}
		return output, nil
	case "change_outcome_unknown":
		if !validChangeResultID(output.ChangeID) || output.ResourceID != "" ||
			output.ErrorCode != changebiz.FailureOutcomeUnknown {
			return parsedToolResult{}, errors.New("uncertain change tool result is invalid")
		}
		return output, nil
	case "invalid_input":
		// 执行详情只展示稳定事实；具体参数修正原因仅保留在 Eino 循环内。
		output.Summary = "工具参数无效，已将修正原因返回模型"
		return output, nil
	case "not_found":
		output.Summary = "工具查询目标已不存在，已返回模型重新定位"
		return output, nil
	default:
		return parsedToolResult{}, fmt.Errorf(
			"assistant tool result has unsupported status %q",
			output.Status,
		)
	}
}

func changeState(status string) changebiz.State {
	switch status {
	case "change_succeeded":
		return changebiz.StateSucceeded
	case "change_failed":
		return changebiz.StateFailed
	case "change_outcome_unknown":
		return changebiz.StateOutcomeUnknown
	default:
		return ""
	}
}

func validChangeResultID(id string) bool {
	return uuid.Validate(id) == nil
}

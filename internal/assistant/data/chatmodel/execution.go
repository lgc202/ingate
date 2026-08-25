package chatmodel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"

	runbiz "github.com/lgc202/ingate/internal/assistant/biz/run"
)

// executionMiddleware 使用 Eino 官方中间件记录真正发生的模型与工具调用。
// 它按 Run 创建，不在并发 Run 之间共享模型调用状态。
type executionMiddleware struct {
	modelName     string
	recorder      runbiz.ExecutionRecorder
	authorizeTool func(string) error
	modelCallID   string
}

func newExecutionMiddleware(
	modelName string,
	recorder runbiz.ExecutionRecorder,
	authorizeTool func(string) error,
) adk.AgentMiddleware {
	middleware := &executionMiddleware{
		modelName:     modelName,
		recorder:      recorder,
		authorizeTool: authorizeTool,
	}
	return adk.AgentMiddleware{
		BeforeChatModel: middleware.beforeChatModel,
		AfterChatModel:  middleware.afterChatModel,
		WrapToolCall: compose.ToolMiddleware{
			Invokable: middleware.wrapToolCall,
		},
	}
}

func (m *executionMiddleware) beforeChatModel(ctx context.Context, _ *adk.ChatModelAgentState) error {
	callID := uuid.NewString()
	if err := m.recorder.ModelStarted(ctx, callID, m.modelName); err != nil {
		return err
	}
	m.modelCallID = callID
	return nil
}

func (m *executionMiddleware) afterChatModel(
	ctx context.Context,
	state *adk.ChatModelAgentState,
) error {
	message := state.Messages[len(state.Messages)-1]
	summary := "模型已生成回答"
	if len(message.ToolCalls) > 0 {
		summary = fmt.Sprintf("模型选择了 %d 个工具", len(message.ToolCalls))
	}
	if err := m.recorder.ModelCompleted(ctx, m.modelCallID, summary); err != nil {
		return err
	}
	m.modelCallID = ""
	return nil
}

func (m *executionMiddleware) wrapToolCall(
	next compose.InvokableToolEndpoint,
) compose.InvokableToolEndpoint {
	return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		callID := input.CallID
		if callID == "" {
			// 外部模型没有提供调用 ID 时生成内部关联标识，保证执行步骤仍可唯一结束。
			callID = uuid.NewString()
		}
		if err := m.recorder.ToolStarted(ctx, callID, input.Name); err != nil {
			return nil, err
		}
		if err := m.authorizeTool(input.Name); err != nil {
			return nil, m.failTool(
				ctx,
				callID,
				input.Name,
				fmt.Errorf("authorize assistant tool: %w", err),
			)
		}

		output, err := next(ctx, input)
		if err != nil {
			return nil, m.failTool(ctx, callID, input.Name, err)
		}
		summary, err := toolResultSummary(output.Result)
		if err != nil {
			return nil, m.failTool(ctx, callID, input.Name, err)
		}
		if err := m.recorder.ToolCompleted(ctx, callID, summary); err != nil {
			return nil, err
		}
		return output, nil
	}
}

func (m *executionMiddleware) failTool(
	ctx context.Context,
	callID string,
	name string,
	cause error,
) error {
	recordErr := m.recorder.ToolFailed(ctx, callID)
	return errors.Join(
		runbiz.ErrToolUnavailable,
		fmt.Errorf("run assistant tool %q: %w", name, cause),
		recordErr,
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

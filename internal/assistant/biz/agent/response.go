package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	agenttool "github.com/lgc202/ingate/internal/assistant/biz/agent/tool"
)

const maxResponseBytes = 8 << 20

// responseCollector 消费 Eino AgentEvent，并汇总最终回答及厂商显式返回的思考内容。
// 工具选择消息和工具结果只服务于当前循环，不会成为用户可见消息。
type responseCollector struct {
	content   strings.Builder
	reasoning strings.Builder
	events    EventSink
	interrupt *ApprovalInterruption
}

func (c *responseCollector) consume(ctx context.Context, event *adk.AgentEvent) error {
	if event == nil {
		return errors.New("agent returned a nil event")
	}
	if event.Err != nil {
		if errors.Is(event.Err, adk.ErrExceedMaxIterations) {
			return fmt.Errorf("%w: %w", ErrIterationLimit, event.Err)
		}
		return fmt.Errorf("execute Eino agent: %w", event.Err)
	}
	interrupt, err := validateAction(event.Action)
	if err != nil {
		return err
	}
	if interrupt != nil {
		if c.interrupt != nil {
			return errors.New("agent returned more than one approval interruption")
		}
		c.interrupt = interrupt
	}
	if event.Output == nil || event.Output.MessageOutput == nil {
		return nil
	}
	return c.appendOutput(ctx, event.Output.MessageOutput)
}

func validateAction(action *adk.AgentAction) (*ApprovalInterruption, error) {
	if action == nil || action.Exit || action.BreakLoop != nil {
		return nil, nil
	}
	if action.Interrupted != nil {
		return approvalInterruption(action.Interrupted)
	}
	if action.TransferToAgent != nil {
		return nil, fmt.Errorf(
			"eino agent requested transfer to unsupported agent %q",
			action.TransferToAgent.DestAgentName,
		)
	}
	if action.CustomizedAction != nil {
		return nil, errors.New("eino agent returned an unsupported custom action")
	}
	return nil, nil
}

func approvalInterruption(info *adk.InterruptInfo) (*ApprovalInterruption, error) {
	if info == nil {
		return nil, errors.New("agent returned a nil interruption")
	}
	var result *ApprovalInterruption
	for _, interrupt := range info.InterruptContexts {
		if interrupt == nil || !interrupt.IsRootCause {
			continue
		}
		request, ok := interrupt.Info.(*agenttool.ApprovalRequest)
		if !ok || request == nil {
			return nil, fmt.Errorf(
				"agent returned unsupported root interruption info %T",
				interrupt.Info,
			)
		}
		if result != nil {
			return nil, errors.New("agent returned multiple root approval interruptions")
		}
		result = &ApprovalInterruption{InterruptID: interrupt.ID, Request: *request}
	}
	if result == nil {
		return nil, errors.New("agent interruption contains no root approval request")
	}
	return result, nil
}

func (c *responseCollector) appendOutput(ctx context.Context, output *adk.MessageVariant) error {
	if !output.IsStreaming {
		return c.append(ctx, output.Message)
	}
	if output.MessageStream == nil {
		return errors.New("agent returned a nil message stream")
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
		if err := c.append(ctx, chunk); err != nil {
			return err
		}
	}
}

func (c *responseCollector) append(ctx context.Context, message *schema.Message) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if message == nil {
		return errors.New("agent returned a nil message")
	}
	if message.Role != schema.Assistant || len(message.ToolCalls) > 0 {
		return nil
	}
	if err := c.appendReasoning(ctx, message.ReasoningContent); err != nil {
		return err
	}
	return c.appendContent(ctx, message.Content)
}

func (c *responseCollector) appendReasoning(ctx context.Context, delta string) error {
	if delta == "" {
		return nil
	}
	if len(delta) > maxResponseBytes-c.reasoning.Len() {
		return errors.New("assistant model reasoning exceeds the response size limit")
	}
	c.reasoning.WriteString(delta)
	// 事件接收器是同步边界：若执行事实已不可写，应停止继续消耗模型流，避免页面
	// 展示一段最终无法归属到当前执行的回答。
	if err := c.events.Emit(ctx, ReasoningDelta{Content: delta}); err != nil {
		return fmt.Errorf("record model reasoning delta: %w", err)
	}
	return nil
}

func (c *responseCollector) appendContent(ctx context.Context, delta string) error {
	if delta == "" {
		return nil
	}
	if len(delta) > maxResponseBytes-c.content.Len() {
		return errors.New("assistant model content exceeds the response size limit")
	}
	c.content.WriteString(delta)
	if err := c.events.Emit(ctx, ContentDelta{Content: delta}); err != nil {
		return fmt.Errorf("record model content delta: %w", err)
	}
	return nil
}

func (c *responseCollector) build() Response {
	return Response{
		Content:          c.content.String(),
		ReasoningContent: c.reasoning.String(),
		Interruption:     c.interrupt,
	}
}

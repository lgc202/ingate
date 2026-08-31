package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

const maxResponseBytes = 8 << 20

// responseCollector 消费 Eino AgentEvent，并汇总最终回答及厂商显式返回的思考内容。
// 工具选择消息和工具结果只服务于当前循环，不会成为用户可见消息。
type responseCollector struct {
	content   strings.Builder
	reasoning strings.Builder
	events    EventSink
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
	if err := validateAction(event.Action); err != nil {
		return err
	}
	if event.Output == nil || event.Output.MessageOutput == nil {
		return nil
	}
	return c.appendOutput(ctx, event.Output.MessageOutput)
}

func validateAction(action *adk.AgentAction) error {
	if action == nil || action.Exit || action.BreakLoop != nil {
		return nil
	}
	// 当前运维助手是单 Agent，且产品协议尚未提供人工确认与恢复入口。
	// 与其静默丢弃 ADK Action，不如在能力边界处明确失败。
	if action.Interrupted != nil {
		return errors.New("eino agent requested interruption without checkpoint support")
	}
	if action.TransferToAgent != nil {
		return fmt.Errorf(
			"eino agent requested transfer to unsupported agent %q",
			action.TransferToAgent.DestAgentName,
		)
	}
	if action.CustomizedAction != nil {
		return errors.New("eino agent returned an unsupported custom action")
	}
	return nil
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
	}
}

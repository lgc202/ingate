package operations

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	agentprotocol "github.com/lgc202/ingate/internal/assistant/agent"
)

// responseBuilder 汇总最终回答及厂商显式返回的思考内容。
// 工具选择消息和工具结果只服务于当前循环，不会成为用户可见消息。
type responseBuilder struct {
	content   strings.Builder
	reasoning strings.Builder
	events    agentprotocol.EventSink
}

func (b *responseBuilder) append(ctx context.Context, message *schema.Message) error {
	if message.Role != schema.Assistant || len(message.ToolCalls) > 0 {
		return nil
	}
	if err := b.appendReasoning(ctx, message.ReasoningContent); err != nil {
		return err
	}
	return b.appendContent(ctx, message.Content)
}

func (b *responseBuilder) appendReasoning(ctx context.Context, delta string) error {
	if delta == "" {
		return nil
	}
	b.reasoning.WriteString(delta)
	// 事件接收器是同步边界：若执行事实已不可写，应停止继续消耗模型流，避免页面
	// 展示一段最终无法归属到当前执行的回答。
	if err := b.events.Emit(ctx, agentprotocol.ReasoningDelta{Content: delta}); err != nil {
		return fmt.Errorf("record model reasoning delta: %w", err)
	}
	return nil
}

func (b *responseBuilder) appendContent(ctx context.Context, delta string) error {
	if delta == "" {
		return nil
	}
	b.content.WriteString(delta)
	if err := b.events.Emit(ctx, agentprotocol.ContentDelta{Content: delta}); err != nil {
		return fmt.Errorf("record model content delta: %w", err)
	}
	return nil
}

func (b *responseBuilder) build() agentprotocol.Response {
	return agentprotocol.Response{
		Content:          b.content.String(),
		ReasoningContent: b.reasoning.String(),
	}
}

package agent

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	executionbiz "github.com/lgc202/ingate/internal/assistant/biz/execution"
)

// replyBuilder 汇总同一次模型调用的推理与回答，并同步发送增量事件。
// 工具选择消息和工具结果只供 Eino 继续推理，不会进入用户可见回复。
type replyBuilder struct {
	content   strings.Builder
	reasoning strings.Builder
	emit      func(executionbiz.Delta) error
}

func (b *replyBuilder) append(message *schema.Message) error {
	if message.Role != schema.Assistant || len(message.ToolCalls) > 0 {
		return nil
	}
	if err := b.appendDelta(executionbiz.DeltaReasoning, message.ReasoningContent); err != nil {
		return err
	}
	return b.appendDelta(executionbiz.DeltaContent, message.Content)
}

func (b *replyBuilder) appendDelta(deltaType executionbiz.DeltaType, delta string) error {
	if delta == "" {
		return nil
	}
	if deltaType == executionbiz.DeltaReasoning {
		b.reasoning.WriteString(delta)
	} else {
		b.content.WriteString(delta)
	}
	if err := b.emit(executionbiz.Delta{Type: deltaType, Content: delta}); err != nil {
		return fmt.Errorf("emit model delta: %w", err)
	}
	return nil
}

func (b *replyBuilder) result() executionbiz.AgentResult {
	return executionbiz.AgentResult{
		Content:          b.content.String(),
		ReasoningContent: b.reasoning.String(),
	}
}

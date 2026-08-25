package chatmodel

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	runbiz "github.com/lgc202/ingate/internal/assistant/biz/run"
)

// replyBuilder 汇总同一次模型调用的推理与回答，并同步发送增量事件。
// 工具选择消息和工具结果只供 Eino 继续推理，不会进入用户可见回复。
type replyBuilder struct {
	content   strings.Builder
	reasoning strings.Builder
	emit      func(runbiz.Delta) error
}

func (b *replyBuilder) append(message *schema.Message) error {
	if message.Role != schema.Assistant || len(message.ToolCalls) > 0 {
		return nil
	}
	if err := b.appendDelta(runbiz.DeltaReasoning, message.ReasoningContent); err != nil {
		return err
	}
	return b.appendDelta(runbiz.DeltaContent, message.Content)
}

func (b *replyBuilder) appendDelta(deltaType runbiz.DeltaType, delta string) error {
	if delta == "" {
		return nil
	}
	if deltaType == runbiz.DeltaReasoning {
		b.reasoning.WriteString(delta)
	} else {
		b.content.WriteString(delta)
	}
	if err := b.emit(runbiz.Delta{Type: deltaType, Content: delta}); err != nil {
		return fmt.Errorf("emit model delta: %w", err)
	}
	return nil
}

func (b *replyBuilder) result() runbiz.Result {
	return runbiz.Result{
		Content:          b.content.String(),
		ReasoningContent: b.reasoning.String(),
	}
}

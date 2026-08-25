// Package chatmodel 通过 Eino ADK 执行运维助手模型。
package chatmodel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	modelbiz "github.com/lgc202/ingate/internal/assistant/biz/model"
)

const (
	maxIterations = 8
	instruction   = "你是 Ingate 运维助手。回答必须基于可验证的系统事实；需要变更时先说明影响并等待用户确认。"
)

// Agent 在每次 Run 开始时读取当前模型连接。
// 这样新配置无需重启进程，已开始的 Run 仍使用自己的配置快照。
type Agent struct {
	connections *modelbiz.Service
}

// ChatModel 封装一次 Run 固定使用的 Eino Runner。
type ChatModel struct {
	name string
	eino *adk.Runner
}

// replyBuilder 汇总同一次模型调用的推理与回答，并同步发送增量事件。
type replyBuilder struct {
	content   strings.Builder
	reasoning strings.Builder
	emit      func(conversation.ModelDelta) error
}

// NewAgent 创建模型选取器，不在进程启动时固化在线业务配置。
func NewAgent(connections *modelbiz.Service) *Agent {
	return &Agent{connections: connections}
}

// Model 选取当前配置并创建一个不可变的模型执行对象。
func (a *Agent) Model(ctx context.Context) (conversation.Model, error) {
	connection, err := a.connections.ForRun(ctx)
	if errors.Is(err, modelbiz.ErrNotConfigured) {
		return nil, conversation.ErrModelNotConfigured
	}
	if err != nil {
		return nil, fmt.Errorf("load assistant model connection: %w", err)
	}
	model, err := newChatModel(ctx, connection)
	if err != nil {
		return nil, err
	}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "ingate_operations_assistant",
		Description:   "Helps operators understand and operate the current Ingate environment",
		Instruction:   instruction,
		Model:         model,
		MaxIterations: maxIterations,
	})
	if err != nil {
		return nil, fmt.Errorf("create Eino chat model agent: %w", err)
	}
	return &ChatModel{
		name: connection.Model,
		eino: adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true}),
	}, nil
}

// Name 返回该 Run 实际使用的模型名称。
func (m *ChatModel) Name() string {
	return m.name
}

// Generate 根据持久消息生成回复，并把模型增量交给调用方发送和暂存。
func (m *ChatModel) Generate(
	ctx context.Context,
	history []conversation.Message,
	emit func(conversation.ModelDelta) error,
) (conversation.ModelResult, error) {
	messages := make([]adk.Message, 0, len(history))
	for _, message := range history {
		switch message.Role {
		case conversation.RoleUser:
			messages = append(messages, schema.UserMessage(message.Content))
		case conversation.RoleAssistant:
			messages = append(messages, schema.AssistantMessage(message.Content, nil))
		}
	}
	reply := replyBuilder{emit: emit}
	iterator := m.eino.Run(ctx, messages)
	for event, ok := iterator.Next(); ok; event, ok = iterator.Next() {
		if event.Err != nil {
			return conversation.ModelResult{}, fmt.Errorf("run Eino agent: %w", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		output := event.Output.MessageOutput
		if !output.IsStreaming {
			if err := reply.append(output.Message); err != nil {
				return conversation.ModelResult{}, err
			}
			continue
		}
		stream := output.MessageStream
		for {
			chunk, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				stream.Close()
				return conversation.ModelResult{}, fmt.Errorf("read Eino model stream: %w", err)
			}
			if err := reply.append(chunk); err != nil {
				stream.Close()
				return conversation.ModelResult{}, err
			}
		}
		stream.Close()
	}
	return reply.result(), nil
}

func (b *replyBuilder) append(message *schema.Message) error {
	if err := b.appendDelta(conversation.ModelDeltaReasoning, message.ReasoningContent); err != nil {
		return err
	}
	return b.appendDelta(conversation.ModelDeltaContent, message.Content)
}

func (b *replyBuilder) appendDelta(
	deltaType conversation.ModelDeltaType,
	delta string,
) error {
	if delta == "" {
		return nil
	}
	if deltaType == conversation.ModelDeltaReasoning {
		b.reasoning.WriteString(delta)
	} else {
		b.content.WriteString(delta)
	}
	if err := b.emit(conversation.ModelDelta{Type: deltaType, Content: delta}); err != nil {
		return fmt.Errorf("emit model delta: %w", err)
	}
	return nil
}

func (b *replyBuilder) result() conversation.ModelResult {
	return conversation.ModelResult{
		Content:          b.content.String(),
		ReasoningContent: b.reasoning.String(),
	}
}

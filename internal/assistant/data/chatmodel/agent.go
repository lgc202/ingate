// Package chatmodel 通过 Eino ADK 执行运维助手模型。
package chatmodel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	"github.com/lgc202/ingate/internal/assistant/conf"
)

// Agent 封装 Eino Runner，后续工具和审批仍由同一个 ADK 执行链扩展。
type Agent struct {
	model  string
	runner *adk.Runner
}

// NewAgent 创建兼容 OpenAI Chat Completions 协议的 Eino Agent。
func NewAgent(ctx context.Context, config *conf.Model) (*Agent, error) {
	if strings.TrimSpace(config.GetBaseUrl()) == "" && strings.TrimSpace(config.GetName()) == "" {
		return &Agent{}, nil
	}
	maxOutputTokens := int(config.GetMaxOutputTokens())
	model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey: config.GetApiKey(), BaseURL: config.GetBaseUrl(), Model: config.GetName(),
		Timeout: config.GetTimeout().AsDuration(), MaxCompletionTokens: &maxOutputTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("create Eino chat model: %w", err)
	}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "ingate_operations_assistant",
		Description:   "Helps operators understand and operate the current Ingate environment",
		Instruction:   config.GetInstruction(),
		Model:         model,
		MaxIterations: int(config.GetMaxIterations()),
	})
	if err != nil {
		return nil, fmt.Errorf("create Eino chat model agent: %w", err)
	}
	return &Agent{
		model:  config.GetName(),
		runner: adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true}),
	}, nil
}

func (a *Agent) Model() string {
	return a.model
}

func (a *Agent) Generate(
	ctx context.Context,
	history []conversation.Message,
	emit func(string) error,
) (string, error) {
	if a.runner == nil {
		return "", conversation.ErrModelNotConfigured
	}
	messages := make([]adk.Message, 0, len(history))
	for _, message := range history {
		switch message.Role {
		case conversation.RoleUser:
			messages = append(messages, schema.UserMessage(message.Content))
		case conversation.RoleAssistant:
			messages = append(messages, schema.AssistantMessage(message.Content, nil))
		}
	}
	var content strings.Builder
	iterator := a.runner.Run(ctx, messages)
	for event, ok := iterator.Next(); ok; event, ok = iterator.Next() {
		if event.Err != nil {
			return "", fmt.Errorf("run Eino agent: %w", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		output := event.Output.MessageOutput
		if !output.IsStreaming {
			if err := appendDelta(&content, output.Message.Content, emit); err != nil {
				return "", err
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
				return "", fmt.Errorf("read Eino model stream: %w", err)
			}
			if err := appendDelta(&content, chunk.Content, emit); err != nil {
				stream.Close()
				return "", err
			}
		}
		stream.Close()
	}
	return content.String(), nil
}

func appendDelta(content *strings.Builder, delta string, emit func(string) error) error {
	if delta == "" {
		return nil
	}
	content.WriteString(delta)
	if err := emit(delta); err != nil {
		return fmt.Errorf("emit model delta: %w", err)
	}
	return nil
}

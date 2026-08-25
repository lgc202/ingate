package chatmodel

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"

	modelbiz "github.com/lgc202/ingate/internal/assistant/biz/model"
)

// newChatModel 只处理模型协议差异。直连或经过 Ingate 的网络路径已经由
// Connection.Endpoint 确定，不应再进入 Agent 执行逻辑。
func newChatModel(
	ctx context.Context,
	connection modelbiz.Connection,
) (einomodel.ToolCallingChatModel, error) {
	switch connection.Protocol {
	case modelbiz.ProtocolOpenAICompatible:
		return newOpenAICompatibleModel(ctx, connection)
	case modelbiz.ProtocolAnthropic:
		return newAnthropicModel(ctx, connection)
	default:
		return nil, fmt.Errorf("unsupported assistant model protocol %d", connection.Protocol)
	}
}

func newOpenAICompatibleModel(
	ctx context.Context,
	connection modelbiz.Connection,
) (einomodel.ToolCallingChatModel, error) {
	maxOutputTokens := connection.MaxOutputTokens
	model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:              connection.APIKey,
		BaseURL:             connection.Endpoint,
		Model:               connection.Model,
		Timeout:             connection.Timeout,
		MaxCompletionTokens: &maxOutputTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("create OpenAI-compatible chat model: %w", err)
	}
	return model, nil
}

func newAnthropicModel(
	ctx context.Context,
	connection modelbiz.Connection,
) (einomodel.ToolCallingChatModel, error) {
	config := &claude.Config{
		APIKey:    connection.APIKey,
		BaseURL:   &connection.Endpoint,
		Model:     connection.Model,
		MaxTokens: connection.MaxOutputTokens,
		HTTPClient: &http.Client{
			Timeout: connection.Timeout,
		},
	}
	if connection.ReasoningBudgetTokens > 0 {
		config.Thinking = &claude.Thinking{
			Enable:       true,
			BudgetTokens: connection.ReasoningBudgetTokens,
		}
	}
	model, err := claude.NewChatModel(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create Anthropic chat model: %w", err)
	}
	return model, nil
}

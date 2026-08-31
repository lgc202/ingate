// Package chatmodel 将 Assistant 模型连接转换为 Eino ChatModel。
package chatmodel

import (
	"context"
	"fmt"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"

	"github.com/lgc202/ingate/internal/assistant/biz/modelconfig"
)

// New 只处理模型协议差异。直连或经过 Ingate 的网络路径已经由
// Connection.Endpoint 确定，不应再进入 Agent 执行逻辑。
func New(
	ctx context.Context,
	connection modelconfig.Connection,
) (einomodel.ToolCallingChatModel, error) {
	switch connection.Protocol {
	case modelconfig.ProtocolOpenAICompatible:
		return newOpenAICompatibleModel(ctx, connection)
	case modelconfig.ProtocolAnthropic:
		return newAnthropicModel(ctx, connection)
	default:
		return nil, fmt.Errorf("unsupported assistant model protocol %d", connection.Protocol)
	}
}

func newOpenAICompatibleModel(
	ctx context.Context,
	connection modelconfig.Connection,
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
	connection modelconfig.Connection,
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
		thinking := anthropic.ThinkingConfigParamOfEnabled(
			int64(connection.ReasoningBudgetTokens),
		)
		config.ThinkingConfig = &thinking
	}
	model, err := claude.NewChatModel(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create Anthropic chat model: %w", err)
	}
	return model, nil
}

package chatcompletion

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

// ObserveOpenAIResponse 从完整的 OpenAI 兼容响应中提取运行信息。
// 解析失败不影响上游响应透传，因此返回值只表达是否获得了新信息。
func ObserveOpenAIResponse(body []byte) (ResponseMetadata, bool) {
	if !gjson.ValidBytes(body) {
		return ResponseMetadata{}, false
	}

	var metadata ResponseMetadata
	model := gjson.GetBytes(body, "model")
	if model.Type == gjson.String && routeconfig.IsValidModelName(model.String()) {
		metadata.ResponseModel = model.String()
	}
	if reason := gjson.GetBytes(body, "choices.0.finish_reason"); reason.Type == gjson.String {
		metadata.FinishReason = reason.String()
	}
	hasMetadata := metadata.ResponseModel != "" || metadata.FinishReason != ""

	promptTokenValue := gjson.GetBytes(body, "usage.prompt_tokens")
	completionTokenValue := gjson.GetBytes(body, "usage.completion_tokens")
	totalTokenValue := gjson.GetBytes(body, "usage.total_tokens")
	promptTokens, hasPromptTokens := tokenCount(promptTokenValue)
	completionTokens, hasCompletionTokens := tokenCount(completionTokenValue)
	totalTokens, hasTotalTokens := tokenCount(totalTokenValue)
	if (promptTokenValue.Exists() && !hasPromptTokens) ||
		(completionTokenValue.Exists() && !hasCompletionTokens) ||
		(totalTokenValue.Exists() && !hasTotalTokens) {
		return metadata, hasMetadata
	}
	if hasPromptTokens || hasCompletionTokens || hasTotalTokens {
		// OpenAI usage 必须同时给出输入和输出 Token；只拿部分字段推导会静默少结算。
		if !hasPromptTokens || !hasCompletionTokens {
			return metadata, hasMetadata
		}
		if promptTokens > math.MaxUint64-completionTokens {
			return metadata, hasMetadata
		}
		calculatedTotal := promptTokens + completionTokens
		if !hasTotalTokens {
			totalTokens = calculatedTotal
		} else if totalTokens != calculatedTotal {
			return metadata, hasMetadata
		}
		metadata.Usage = Usage{
			InputTokens:  promptTokens,
			OutputTokens: completionTokens,
			TotalTokens:  totalTokens,
			Found:        true,
			Final:        true,
		}
	}

	return metadata, hasMetadata || metadata.Usage.Found
}

// RewriteOpenAIResponseModel 把上游响应中的真实模型名恢复为 Route 对外发布的稳定模型名。
func RewriteOpenAIResponseModel(body []byte, clientModel string) ([]byte, bool, error) {
	if !routeconfig.IsValidModelName(clientModel) {
		return nil, false, errors.New("client model is invalid")
	}
	model := gjson.GetBytes(body, "model")
	if model.Type != gjson.String || model.String() == clientModel {
		return body, false, nil
	}
	converted, err := sjson.SetBytes(body, "model", clientModel)
	if err != nil {
		return nil, false, fmt.Errorf("rewrite response model: %w", err)
	}
	return converted, true, nil
}

func tokenCount(value gjson.Result) (uint64, bool) {
	if !value.Exists() || value.Type != gjson.Number {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value.Raw, 10, 64)
	return parsed, err == nil
}

func mergeResponseMetadata(target *ResponseMetadata, update ResponseMetadata) {
	if update.ResponseModel != "" {
		target.ResponseModel = update.ResponseModel
	}
	if update.FinishReason != "" {
		target.FinishReason = update.FinishReason
	}
	if update.Usage.Found {
		target.Usage = update.Usage
	}
}

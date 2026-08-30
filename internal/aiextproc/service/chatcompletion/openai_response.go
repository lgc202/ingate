package chatcompletion

import (
	"errors"
	"fmt"
	"math"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

// ObserveOpenAIResponse 从完整的 OpenAI 兼容响应中提取运行信息
// 解析失败不影响上游响应透传，因此返回值只表达是否获得了新信息。
func ObserveOpenAIResponse(body []byte) (ResponseMetadata, bool) {
	if !gjson.ValidBytes(body) {
		return ResponseMetadata{}, false
	}

	var metadata ResponseMetadata
	if model := gjson.GetBytes(body, "model"); model.Type == gjson.String && routeconfig.IsValidModelName(model.String()) {
		metadata.ResponseModel = model.String()
	}
	if reason := gjson.GetBytes(body, "choices.0.finish_reason"); reason.Type == gjson.String {
		metadata.FinishReason = reason.String()
	}

	input, hasInput := tokenCount(gjson.GetBytes(body, "usage.prompt_tokens"))
	output, hasOutput := tokenCount(gjson.GetBytes(body, "usage.completion_tokens"))
	total, hasTotal := tokenCount(gjson.GetBytes(body, "usage.total_tokens"))
	if hasInput || hasOutput || hasTotal {
		if !hasTotal {
			if input > math.MaxUint64-output {
				return metadata, metadata.ResponseModel != "" || metadata.FinishReason != ""
			}
			total = input + output
		} else if total < input || total < output {
			return metadata, metadata.ResponseModel != "" || metadata.FinishReason != ""
		}
		metadata.Usage = Usage{
			InputTokens:  input,
			OutputTokens: output,
			TotalTokens:  total,
			Found:        true,
			Final:        true,
		}
	}

	return metadata, metadata.ResponseModel != "" || metadata.FinishReason != "" || metadata.Usage.Found
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
	if !value.Exists() || value.Type != gjson.Number || value.Int() < 0 {
		return 0, false
	}
	return value.Uint(), true
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

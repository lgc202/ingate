package chatcompletion

import (
	"errors"
	"fmt"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

// InspectRequest 校验 downstream 请求并提取 Envoy 选择模型线路所需的信息
// downstream 阶段不修改正文，确保主线路和后续重试都能从同一份原始请求重新转换。
func InspectRequest(body []byte) (RequestMetadata, error) {
	if !gjson.ValidBytes(body) {
		return RequestMetadata{}, invalidRequest("request body must be valid JSON")
	}

	model := gjson.GetBytes(body, "model")
	if model.Type != gjson.String || !routeconfig.IsValidModelName(model.String()) {
		return RequestMetadata{}, invalidRequest("model must be a valid non-empty string")
	}
	stream := gjson.GetBytes(body, "stream")
	if stream.Exists() && stream.Type != gjson.True && stream.Type != gjson.False {
		return RequestMetadata{}, invalidRequest("stream must be a boolean")
	}
	return RequestMetadata{Model: model.String(), Streaming: stream.Bool()}, nil
}

// RewriteOpenAIRequest 生成使用厂商真实模型名的 OpenAI 兼容请求。
func RewriteOpenAIRequest(body []byte, upstreamModel string) (UpstreamRequest, error) {
	if !routeconfig.IsValidModelName(upstreamModel) {
		return UpstreamRequest{}, errors.New("upstream model is invalid")
	}
	metadata, err := InspectRequest(body)
	if err != nil {
		return UpstreamRequest{}, err
	}

	mutated := body
	changed := false
	if upstreamModel != metadata.Model {
		mutated, err = sjson.SetBytes(mutated, "model", upstreamModel)
		if err != nil {
			return UpstreamRequest{}, fmt.Errorf("rewrite upstream model: %w", err)
		}
		changed = true
	}
	includeUsage := gjson.GetBytes(mutated, "stream_options.include_usage")
	if metadata.Streaming && includeUsage.Type != gjson.True {
		// OpenAI 兼容流只有显式请求 include_usage 才会在结束事件中返回 Token 用量
		mutated, err = sjson.SetBytes(mutated, "stream_options.include_usage", true)
		if err != nil {
			return UpstreamRequest{}, fmt.Errorf("enable streaming usage: %w", err)
		}
		changed = true
	}

	return UpstreamRequest{Body: mutated, BodyChanged: changed}, nil
}

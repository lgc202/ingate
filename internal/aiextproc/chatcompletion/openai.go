package chatcompletion

import (
	"bytes"
	"fmt"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// OpenAIStream 只缓存尚未结束的 SSE 行，避免把流式响应整体缓冲
type OpenAIStream struct {
	buffer      []byte
	clientModel string
	metadata    ResponseMetadata
}

// NewOpenAIStream 创建一条 OpenAI 兼容响应流的转换状态
// clientModel 是 AI Route 对外发布的稳定模型名，响应不能泄漏 Service 使用的真实模型名
func NewOpenAIStream(clientModel string) *OpenAIStream {
	return &OpenAIStream{clientModel: clientModel}
}

// InspectRequest 校验入口请求并提取 Envoy 选择模型线路所需的信息
// 入口阶段不修改正文，确保主线路和后续重试都能从同一份原始请求重新转换
func InspectRequest(body []byte) (RequestMetadata, error) {
	if !gjson.ValidBytes(body) {
		return RequestMetadata{}, invalidRequest("request body must be valid JSON")
	}

	model := gjson.GetBytes(body, "model")
	if model.Type != gjson.String || model.String() == "" {
		return RequestMetadata{}, invalidRequest("model must be a non-empty string")
	}
	stream := gjson.GetBytes(body, "stream")
	if stream.Exists() && stream.Type != gjson.True && stream.Type != gjson.False {
		return RequestMetadata{}, invalidRequest("stream must be a boolean")
	}
	return RequestMetadata{Model: model.String(), Streaming: stream.Bool()}, nil
}

// RewriteOpenAIRequest 生成使用厂商真实模型名的 OpenAI 兼容请求
func RewriteOpenAIRequest(body []byte, upstreamModel string) (UpstreamRequest, error) {
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

// ObserveOpenAIResponse 从完整的 OpenAI 兼容响应中提取运行信息
// 解析失败不影响上游响应透传，因此返回值只表达是否获得了新信息
func ObserveOpenAIResponse(body []byte) (ResponseMetadata, bool) {
	if !gjson.ValidBytes(body) {
		return ResponseMetadata{}, false
	}

	var metadata ResponseMetadata
	if model := gjson.GetBytes(body, "model"); model.Type == gjson.String {
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
			total = input + output
		}
		metadata.Usage = Usage{
			InputTokens:  input,
			OutputTokens: output,
			TotalTokens:  total,
			Found:        true,
		}
	}

	return metadata, metadata.ResponseModel != "" || metadata.FinishReason != "" || metadata.Usage.Found
}

// RewriteOpenAIResponseModel 把上游响应中的真实模型名恢复为 Route 对外发布的稳定模型名
func RewriteOpenAIResponseModel(body []byte, clientModel string) ([]byte, bool, error) {
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

// Convert 增量读取 OpenAI SSE，提取运行信息并恢复客户端模型名
// ExtProc chunk 与 SSE 行没有边界关系，因此不完整行必须留到下一个 chunk 再处理
func (s *OpenAIStream) Convert(chunk []byte, endOfStream bool) ([]byte, ResponseMetadata, bool, error) {
	s.buffer = append(s.buffer, chunk...)
	var converted []byte
	metadataChanged := false
	for {
		lineEnd := bytes.IndexByte(s.buffer, '\n')
		if lineEnd < 0 {
			break
		}
		line := s.buffer[:lineEnd]
		s.buffer = s.buffer[lineEnd+1:]
		output, changed, err := s.convertLine(line)
		if err != nil {
			return nil, ResponseMetadata{}, false, err
		}
		converted = append(converted, output...)
		converted = append(converted, '\n')
		metadataChanged = changed || metadataChanged
	}
	if endOfStream && len(s.buffer) > 0 {
		output, changed, err := s.convertLine(s.buffer)
		if err != nil {
			return nil, ResponseMetadata{}, false, err
		}
		converted = append(converted, output...)
		metadataChanged = changed || metadataChanged
		s.buffer = nil
	}
	return converted, s.metadata, metadataChanged, nil
}

func (s *OpenAIStream) convertLine(line []byte) ([]byte, bool, error) {
	carriageReturn := bytes.HasSuffix(line, []byte{'\r'})
	line = bytes.TrimSuffix(line, []byte{'\r'})
	if !bytes.HasPrefix(line, []byte("data:")) {
		if carriageReturn {
			line = append(line, '\r')
		}
		return line, false, nil
	}
	payload := bytes.TrimSpace(line[len("data:"):])
	if bytes.Equal(payload, []byte("[DONE]")) {
		return line, false, nil
	}

	// OpenAI SSE 的 data 内容与非流式响应复用相同的 model、choices 和 usage 路径
	metadata, changed := ObserveOpenAIResponse(payload)
	if changed {
		mergeResponseMetadata(&s.metadata, metadata)
	}
	converted, bodyChanged, err := RewriteOpenAIResponseModel(payload, s.clientModel)
	if err != nil {
		return nil, false, err
	}
	if bodyChanged {
		line = append([]byte("data: "), converted...)
	}
	if carriageReturn {
		line = append(line, '\r')
	}
	return line, changed, nil
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

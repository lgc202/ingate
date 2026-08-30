package service

import (
	"bytes"
	"fmt"
	"strconv"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3http "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"github.com/lgc202/ingate/internal/aiextproc/service/chatcompletion"
	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
)

// handleDownstreamResponseHeaders 决定最终响应是否需要流式转换
// upstream ExtProc 流只转换请求，不会进入本文件的响应阶段
func (s *streamState) handleDownstreamResponseHeaders(headers *corev3.HeaderMap) *extprocv3.ProcessingResponse {
	selected, selectedOK := s.request.selectedService()
	request, requestOK := s.request.requestMetadata()
	if !selectedOK || !requestOK {
		// 未经过模型线路的响应保持原样，不能把普通 HTTP 响应误当作模型协议处理
		return headersResponse(responseMessage, nil)
	}
	statusCode, _ := strconv.Atoi(headerValue(headers, ":status"))
	s.responseSuccessful = statusCode >= 200 && statusCode < 300

	var mutation *extprocv3.HeaderMutation
	if selected.protocol == aiprotocol.UpstreamProtocolAnthropic && s.responseSuccessful {
		// 调用方始终接收 OpenAI 兼容 Content-Type，不感知实际模型厂商
		contentType := "application/json"
		if request.Streaming {
			contentType = "text/event-stream"
		}
		mutation = &extprocv3.HeaderMutation{
			SetHeaders: []*corev3.HeaderValueOption{setHeader("content-type", contentType)},
		}
		if request.Streaming {
			mutation.RemoveHeaders = []string{"content-length"}
			s.anthropicStream = chatcompletion.NewAnthropicStream(request.Model)
		}
	}
	if selected.protocol == aiprotocol.UpstreamProtocolOpenAI && request.Streaming && s.responseSuccessful {
		s.openAIStream = chatcompletion.NewOpenAIStream(request.Model)
	}

	response := headersResponse(responseMessage, mutation)
	if request.Streaming && s.responseSuccessful {
		// 流式响应不能整体缓冲，否则首 Token 延迟会退化为完整响应耗时
		// 错误响应通常是普通 JSON，必须保持 BUFFERED 才能完整转换或原样透传
		response.ModeOverride = &extprocv3http.ProcessingMode{
			ResponseBodyMode: extprocv3http.ProcessingMode_STREAMED,
		}
	}
	return response
}

// handleDownstreamResponseBody 按 Envoy 最终选中的模型 Service 观察或转换响应
// OpenAI 兼容响应恢复客户端模型名，Anthropic 响应转换为调用方请求的 Chat Completions 格式
func (s *streamState) handleDownstreamResponseBody(body *extprocv3.HttpBody) (*extprocv3.ProcessingResponse, error) {
	selected, ok := s.request.selectedService()
	if !ok {
		return bodyResponse(responseMessage, nil, nil), nil
	}
	switch selected.protocol {
	case aiprotocol.UpstreamProtocolOpenAI:
		return s.handleDownstreamOpenAIResponse(body)
	case aiprotocol.UpstreamProtocolAnthropic:
		// Anthropic 线路必须把完整响应或 SSE 事件转换成调用方约定的 OpenAI 格式
		return s.handleDownstreamAnthropicResponse(body)
	default:
		return nil, fmt.Errorf("unsupported upstream protocol %q", selected.protocol)
	}
}

func (s *streamState) handleDownstreamOpenAIResponse(body *extprocv3.HttpBody) (*extprocv3.ProcessingResponse, error) {
	if !s.responseSuccessful {
		// 上游或中间代理的错误响应不一定是 SSE 或 Chat Completions 结构，应原样透传。
		return bodyResponse(responseMessage, nil, nil), nil
	}
	request, _ := s.request.requestMetadata()
	if request.Streaming {
		if s.openAIStream == nil {
			s.openAIStream = chatcompletion.NewOpenAIStream(request.Model)
		}
		converted, metadata, metadataChanged, err := s.openAIStream.Convert(body.GetBody(), body.GetEndOfStream())
		if err != nil {
			return nil, err
		}
		if metadataChanged {
			s.responseMetadata = metadata
			s.settleQuota()
		}
		// 即使当前 chunk 只有半行，也必须用空 mutation 吞掉，不能泄漏真实模型名
		response := bodyResponse(responseMessage, &extprocv3.BodyMutation{
			Mutation: &extprocv3.BodyMutation_Body{Body: converted},
		}, nil)
		if metadataChanged {
			response.DynamicMetadata = s.dynamicMetadata()
		}
		return response, nil
	}
	if !body.GetEndOfStream() {
		return nil, errResponseNotBuffered
	}

	metadata, metadataChanged := chatcompletion.ObserveOpenAIResponse(body.GetBody())
	if metadataChanged {
		s.responseMetadata = metadata
		s.settleQuota()
	}
	converted, bodyChanged, err := chatcompletion.RewriteOpenAIResponseModel(body.GetBody(), request.Model)
	if err != nil {
		return nil, err
	}
	var mutation *extprocv3.BodyMutation
	var headers *extprocv3.HeaderMutation
	if bodyChanged {
		mutation = &extprocv3.BodyMutation{
			Mutation: &extprocv3.BodyMutation_Body{Body: converted},
		}
		headers = contentLengthMutation(len(converted))
	}
	response := bodyResponse(responseMessage, mutation, headers)
	if metadataChanged {
		response.DynamicMetadata = s.dynamicMetadata()
	}
	return response, nil
}

func (s *streamState) handleDownstreamAnthropicResponse(
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, error) {
	if !s.responseSuccessful {
		return handleDownstreamAnthropicError(body)
	}

	request, _ := s.request.requestMetadata()
	if request.Streaming {
		if s.anthropicStream == nil {
			s.anthropicStream = chatcompletion.NewAnthropicStream(request.Model)
		}
		// ExtProc chunk 不保证按 SSE 事件边界切分，
		// 转换器负责拼接残片后再输出完整事件。
		converted, metadata, changed, err := s.anthropicStream.Convert(body.GetBody(), body.GetEndOfStream())
		if err != nil {
			return nil, err
		}
		if changed {
			s.responseMetadata = metadata
			s.settleQuota()
		}
		// 不完整的 SSE 事件也要用空 mutation 吞掉，不能向客户端泄漏 Anthropic 原文
		response := bodyResponse(responseMessage, &extprocv3.BodyMutation{
			Mutation: &extprocv3.BodyMutation_Body{Body: converted},
		}, nil)
		if changed {
			response.DynamicMetadata = s.dynamicMetadata()
		}
		return response, nil
	}

	if !body.GetEndOfStream() {
		// 非流式 Anthropic 响应需要完整 JSON，Envoy 必须为 downstream filter 使用 BUFFERED 模式
		return nil, errResponseNotBuffered
	}
	converted, metadata, err := chatcompletion.RewriteAnthropicResponse(body.GetBody(), request.Model)
	if err != nil {
		return nil, err
	}
	s.responseMetadata = metadata
	s.settleQuota()
	var mutation *extprocv3.BodyMutation
	var headers *extprocv3.HeaderMutation
	if !bytes.Equal(converted, body.GetBody()) {
		mutation = &extprocv3.BodyMutation{
			Mutation: &extprocv3.BodyMutation_Body{Body: converted},
		}
		headers = contentLengthMutation(len(converted))
	}
	response := bodyResponse(responseMessage, mutation, headers)
	response.DynamicMetadata = s.dynamicMetadata()
	return response, nil
}

func (s *streamState) settleQuota() {
	usage := s.responseMetadata.Usage
	if !usage.Found || !usage.Final {
		return
	}
	s.processor.settleQuota(s.ctx, s.request, usage.TotalTokens)
}

func handleDownstreamAnthropicError(body *extprocv3.HttpBody) (*extprocv3.ProcessingResponse, error) {
	if !body.GetEndOfStream() {
		return nil, errResponseNotBuffered
	}
	converted, changed, err := chatcompletion.RewriteAnthropicErrorResponse(body.GetBody())
	if err != nil {
		return nil, err
	}
	if !changed {
		// 中间代理生成的 HTML 或普通 JSON 错误不属于 Anthropic 协议，保留其状态、Header 和正文
		return bodyResponse(responseMessage, nil, nil), nil
	}
	return bodyResponse(responseMessage, &extprocv3.BodyMutation{
		Mutation: &extprocv3.BodyMutation_Body{Body: converted},
	}, &extprocv3.HeaderMutation{
		SetHeaders: []*corev3.HeaderValueOption{
			setHeader("content-type", "application/json"),
			setHeader("content-length", strconv.Itoa(len(converted))),
		},
	}), nil
}

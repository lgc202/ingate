package service

import (
	"errors"
	"fmt"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/lgc202/ingate/internal/aiextproc/service/chatcompletion"
	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
)

// handleUpstreamHeaders 在 Envoy 选定 Service 后转换请求
// upstream filter 无需再次接收 Body，直接用 downstream 阶段保存的原文返回 CONTINUE_AND_REPLACE
func (s *streamState) handleUpstreamHeaders(
	headers *corev3.HeaderMap,
	attributes map[string]*structpb.Struct,
) (*extprocv3.ProcessingResponse, error) {
	s.upstream = true
	// 关联 Header 由 downstream filter 注入，并随 Envoy 内部转发保留到 upstream filter
	s.requestID = headerValue(headers, aiprotocol.RequestIDHeader)
	if s.requestID == "" {
		return nil, errors.New("AI upstream request ID is missing")
	}
	request, ok := s.processor.findRequest(s.requestID)
	if !ok {
		return nil, fmt.Errorf("AI request correlation %q was not found", s.requestID)
	}
	s.request = request

	// attributes 来自 Controller 生成的 xDS 元数据，表示本次负载均衡真正选中的线路
	selected, err := selectedModelServiceFromAttributes(
		attributes,
		headerValue(headers, aiprotocol.UpstreamModelHeader),
	)
	if err != nil {
		return nil, err
	}
	apiKey, err := s.processor.apiKeys.APIKey(selected.id)
	if err != nil {
		return nil, err
	}
	// 每次重试都读取客户端原文，不能基于上一条线路已经转换过的 Body 再次转换
	original, ok := request.originalBody()
	if !ok {
		return nil, errors.New("AI original request body is not available")
	}
	converted, err := convertUpstreamRequest(original, selected)
	if err != nil {
		if invalid, ok := errors.AsType[*chatcompletion.InvalidRequestError](err); ok {
			return invalidRequestResponse(invalid.Message()), nil
		}
		return nil, err
	}
	// downstream 响应阶段需要知道最终线路协议，重试还需要恢复基于客户端原文生成的 Body
	retry := request.startUpstreamAttempt(selected)

	replaceBody := converted.BodyChanged || retry
	headerMutation := upstreamHeaderMutation(selected, apiKey, len(converted.Body), replaceBody)
	if !replaceBody {
		// 首次尝试且正文未变化时只清理内部 Header，保留更早 filter 已产生的正文修改
		response := headersResponse(requestMessage, headerMutation)
		response.DynamicMetadata = s.dynamicMetadata()
		return response, nil
	}

	// 重试必须从客户端原文重新生成 Body，不能复用上一条模型线路的协议格式
	response := replaceUpstreamRequest(converted.Body, headerMutation)
	response.DynamicMetadata = s.dynamicMetadata()
	return response, nil
}

// convertUpstreamRequest 根据模型 Service 的协议生成本次上游尝试使用的请求
func convertUpstreamRequest(
	body []byte,
	selected selectedModelService,
) (chatcompletion.UpstreamRequest, error) {
	switch selected.protocol {
	case aiprotocol.UpstreamProtocolOpenAI:
		return chatcompletion.RewriteOpenAIRequest(body, selected.model)
	case aiprotocol.UpstreamProtocolAnthropic:
		return chatcompletion.RewriteAnthropicRequest(body, selected.model)
	default:
		return chatcompletion.UpstreamRequest{}, fmt.Errorf("unsupported upstream protocol %q", selected.protocol)
	}
}

func upstreamHeaderMutation(
	selected selectedModelService,
	apiKey string,
	contentLength int,
	replaceBody bool,
) *extprocv3.HeaderMutation {
	mutation := &extprocv3.HeaderMutation{}
	if replaceBody {
		mutation = contentLengthMutation(contentLength)
	}
	mutation.RemoveHeaders = []string{
		// Ingate 内部关联与选路 Header 只在 Envoy 处理链中使用
		aiprotocol.RequestIDHeader,
		aiprotocol.ModelHeader,
		aiprotocol.UpstreamModelHeader,
		// 不把客户端的压缩协商和访问凭据透传给模型服务
		"accept-encoding",
		"authorization",
		"x-api-key",
	}
	// Envoy AI Gateway 支持 gzip 和 Brotli 解压；当前组件主动请求明文响应，
	// 既保证响应转换和 Token 提取正确，也避免为两个已支持协议引入流式解压状态
	switch selected.protocol {
	case aiprotocol.UpstreamProtocolOpenAI:
		if apiKey != "" {
			mutation.SetHeaders = append(mutation.SetHeaders, setHeader("authorization", "Bearer "+apiKey))
		}
	case aiprotocol.UpstreamProtocolAnthropic:
		// Anthropic 使用固定 Messages 路径和版本 Header，与 OpenAI 兼容端点不同
		mutation.SetHeaders = append(mutation.SetHeaders,
			setHeader(":path", chatcompletion.AnthropicMessagesPath),
			setHeader("anthropic-version", chatcompletion.AnthropicVersion),
			setHeader("content-type", "application/json"),
		)
		if apiKey != "" {
			mutation.SetHeaders = append(mutation.SetHeaders, setHeader("x-api-key", apiKey))
		}
	}
	return mutation
}

package service

import (
	"context"
	"errors"
	"slices"
	"sync"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"github.com/lgc202/ingate/internal/aiextproc/biz/tokenquota"
	"github.com/lgc202/ingate/internal/aiextproc/service/chatcompletion"
)

var (
	errUnsupportedPhase    = errors.New("unsupported ExtProc processing phase")
	errRequestNotBuffered  = errors.New("AI request body requires buffered ExtProc mode")
	errResponseNotBuffered = errors.New("anthropic response body requires buffered ExtProc mode")
)

type callerIdentity struct {
	callerID    string
	accessKeyID string
}

// requestState 只保存同一客户端请求跨 downstream 流、重试流和响应流共享的数据
// 请求正文只在内存中保留到 downstream 流结束，不会写入日志或持久化存储
// Envoy 的每次上游重试都会创建新的 ExtProc 流，因此访问必须由锁保护
type requestState struct {
	mu sync.RWMutex

	body       []byte                         // 调用方提交的原始 OpenAI Chat Completions 请求
	metadata   chatcompletion.RequestMetadata // 从原始请求提取的逻辑模型和流式标记
	clientHost string                         // 进入网关时的请求 Host，不受上游 Host 改写影响
	clientPath string                         // 进入网关时的请求 Path，不含查询参数
	selected   selectedModelService           // Envoy 实际选中的模型 Service 线路
	attempts   int                            // 已成功准备的上游尝试次数，用于识别 Envoy 重试
	identity   callerIdentity                 // 前置鉴权解析出的调用方和访问密钥
	quota      *tokenquota.Session            // 请求开始时通过检查的额度周期
	settled    bool                           // 保证流式响应只结算一次
}

// streamState 保存一条 ExtProc gRPC 流的处理阶段。
// downstream 流覆盖完整客户端请求和最终响应，upstream 流只处理一次上游尝试。
type streamState struct {
	ctx context.Context
	// 流归属与跨流关联
	processor *ExternalProcessor
	upstream  bool
	requestID string
	request   *requestState

	// downstream 响应转换状态，upstream 流不会读写这些字段
	responseMetadata   chatcompletion.ResponseMetadata
	responseSuccessful bool
	openAIStream       *chatcompletion.OpenAIStream
	anthropicStream    *chatcompletion.AnthropicStream
}

func (s *requestState) callerID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.identity.callerID
}

func (s *requestState) setQuotaSession(session *tokenquota.Session) {
	s.mu.Lock()
	s.quota = session
	s.mu.Unlock()
}

func (s *requestState) takeQuotaSession() *tokenquota.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settled {
		return nil
	}
	s.settled = true
	return s.quota
}

func (s *requestState) setClientRequest(host, path string) {
	s.mu.Lock()
	s.clientHost = host
	s.clientPath = path
	s.mu.Unlock()
}

func (s *requestState) setRequest(body []byte, metadata chatcompletion.RequestMetadata) {
	s.mu.Lock()
	s.body = slices.Clone(body)
	s.metadata = metadata
	s.mu.Unlock()
}

func (s *requestState) originalBody() ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.body) == 0 {
		return nil, false
	}
	return slices.Clone(s.body), true
}

func (s *requestState) requestMetadata() (chatcompletion.RequestMetadata, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metadata, s.metadata.Model != ""
}

func (s *requestState) clientRequest() (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clientHost, s.clientPath
}

func (s *requestState) startUpstreamAttempt(selected selectedModelService) bool {
	s.mu.Lock()
	retry := s.attempts > 0
	s.attempts++
	s.selected = selected
	s.mu.Unlock()
	return retry
}

func (s *requestState) selectedService() (selectedModelService, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.selected, s.selected.id != ""
}

func (s *streamState) close() {
	// upstream 流可能早于 downstream 流结束，不能由 upstream 流删除后续重试仍需使用的数据
	if !s.upstream && s.requestID != "" {
		s.processor.deleteRequest(s.requestID)
	}
}

func (s *streamState) handle(request *extprocv3.ProcessingRequest) (*extprocv3.ProcessingResponse, error) {
	// ExtProc 是按阶段交互的协议，每个阶段必须返回对应类型的 ProcessingResponse
	switch request.Request.(type) {
	case *extprocv3.ProcessingRequest_RequestHeaders:
		// upstream filter 会携带 Controller 写入的 Service attributes，downstream filter 不携带
		if request.GetAttributes() != nil {
			return s.handleUpstreamHeaders(request.GetRequestHeaders().GetHeaders(), request.GetAttributes())
		}
		return s.handleDownstreamHeaders(request.GetRequestHeaders().GetHeaders(), request.GetMetadataContext())
	case *extprocv3.ProcessingRequest_RequestBody:
		// upstream filter 使用 CONTINUE_AND_REPLACE 在 Header 阶段直接替换 Body，不接收该阶段
		if s.upstream || s.request == nil {
			return nil, errUnsupportedPhase
		}
		return s.handleDownstreamBody(request.GetRequestBody())
	case *extprocv3.ProcessingRequest_RequestTrailers:
		return trailersResponse(requestMessage), nil
	case *extprocv3.ProcessingRequest_ResponseHeaders:
		// 前置鉴权等过滤器可以直接产生本地响应，此时当前 ExtProc 流不会收到请求阶段
		// 这类响应与模型协议无关，应保持原样返回，不能把正常的 401/403 升级成 500
		if s.request == nil {
			return headersResponse(responseMessage, nil), nil
		}
		// 最终响应只回到 downstream filter，upstream filter 的职责在请求转换后已经结束
		if s.upstream {
			return nil, errUnsupportedPhase
		}
		return s.handleDownstreamResponseHeaders(request.GetResponseHeaders().GetHeaders()), nil
	case *extprocv3.ProcessingRequest_ResponseBody:
		if s.request == nil {
			return bodyResponse(responseMessage, nil, nil), nil
		}
		if s.upstream {
			return nil, errUnsupportedPhase
		}
		return s.handleDownstreamResponseBody(request.GetResponseBody())
	case *extprocv3.ProcessingRequest_ResponseTrailers:
		return trailersResponse(responseMessage), nil
	default:
		return nil, errUnsupportedPhase
	}
}

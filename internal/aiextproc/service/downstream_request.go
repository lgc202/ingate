package service

import (
	"errors"
	"strings"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"github.com/lgc202/ingate/internal/aiextproc/service/chatcompletion"
	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
	"github.com/lgc202/ingate/internal/pkg/extauthz"
)

// handleDownstreamHeaders 为一次客户端请求建立关联标识
// downstream 和每次 upstream 尝试是独立的 ExtProc 流，后续通过该标识共享原始请求
func (s *streamState) handleDownstreamHeaders(
	headers *corev3.HeaderMap,
	metadata *corev3.Metadata,
) (*extprocv3.ProcessingResponse, error) {
	if s.request != nil {
		return nil, errors.New("request headers were processed more than once")
	}

	// Header 阶段先创建关联状态，Body 阶段解析出的模型和原文随后写入同一对象
	s.requestID, s.request = s.processor.registerRequest(identityFromMetadata(metadata))
	host := headerValue(headers, ":authority")
	if host == "" {
		host = headerValue(headers, "host")
	}
	path, _, _ := strings.Cut(headerValue(headers, ":path"), "?")
	// upstream 协议转换会改写 Host 和 Path，必须在任何请求 mutation 发生前保存客户端原始值
	s.request.setClientRequest(host, path)
	mutation := &extprocv3.HeaderMutation{
		SetHeaders: []*corev3.HeaderValueOption{
			setHeader(aiprotocol.RequestIDHeader, s.requestID),
		},
		// 内部模型 Header 必须由请求体生成，不能信任客户端同名输入
		RemoveHeaders: []string{
			aiprotocol.ModelHeader,
			aiprotocol.UpstreamModelHeader,
		},
	}
	return headersResponse(requestMessage, mutation), nil
}

// handleDownstreamBody 在 downstream 阶段提取客户端模型并保留未修改的请求体
// 后续每次上游尝试都从这份原文重新转换，避免重试使用前一厂商的请求格式
func (s *streamState) handleDownstreamBody(body *extprocv3.HttpBody) (*extprocv3.ProcessingResponse, error) {
	if !body.GetEndOfStream() {
		return nil, errRequestNotBuffered
	}
	// downstream 只提取选路需要的信息，不提前转换为任何厂商协议
	metadata, err := chatcompletion.InspectRequest(body.GetBody())
	if err != nil {
		if invalid, ok := errors.AsType[*chatcompletion.InvalidRequestError](err); ok {
			return invalidRequestResponse(invalid.Message()), nil
		}
		return nil, err
	}
	// 保存原文而不是转换结果，保证故障切换到不同协议的 Service 时可以重新生成请求
	s.request.setRequest(body.GetBody(), metadata)
	callerID := s.request.callerID()
	if callerID != "" {
		session, exceeded, err := s.processor.quotas.Begin(s.ctx, callerID, time.Now())
		if err != nil {
			return nil, err
		}
		if exceeded != nil {
			return s.quotaExceededResponse(exceeded), nil
		}
		s.request.setQuotaSession(session)
	}

	response := bodyResponse(requestMessage, nil, &extprocv3.HeaderMutation{
		SetHeaders: []*corev3.HeaderValueOption{setHeader(aiprotocol.ModelHeader, metadata.Model)},
	})
	// Envoy 首次选路时看不到 JSON Body；写入内部模型 Header 后必须重新匹配模型线路
	response.GetRequestBody().GetResponse().ClearRouteCache = true
	// 动态元数据随访问日志输出，ALS 无需读取请求正文即可识别客户端模型
	response.DynamicMetadata = s.dynamicMetadata()
	return response, nil
}

func identityFromMetadata(metadata *corev3.Metadata) callerIdentity {
	fields := metadata.GetFilterMetadata()[extauthz.MetadataNamespace].GetFields()
	return callerIdentity{
		callerID:    fields[extauthz.CallerIDField].GetStringValue(),
		accessKeyID: fields[extauthz.AccessKeyIDField].GetStringValue(),
	}
}

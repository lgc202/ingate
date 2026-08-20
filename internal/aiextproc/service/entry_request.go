package service

import (
	"errors"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"github.com/lgc202/ingate/internal/aiextproc/chatcompletion"
	"github.com/lgc202/ingate/internal/aiextproc/filterconfig"
)

// handleEntryHeaders 为一次客户端请求建立关联标识
// 入口和每次上游尝试是独立的 ExtProc 流，后续通过该标识共享原始请求
func (s *streamState) handleEntryHeaders(headers *corev3.HeaderMap) (*extprocv3.ProcessingResponse, error) {
	if s.request != nil {
		return nil, errors.New("request headers were processed more than once")
	}

	// Header 阶段先创建关联状态，Body 阶段解析出的模型和原文随后写入同一对象
	s.requestID, s.request = s.service.registerRequest()
	host := headerValue(headers, ":authority")
	if host == "" {
		host = headerValue(headers, "host")
	}
	path, _, _ := strings.Cut(headerValue(headers, ":path"), "?")
	// 上游协议转换会改写 Host 和 Path，必须在任何请求 mutation 发生前保存客户端入口值
	s.request.setClientRequest(host, path)
	mutation := &extprocv3.HeaderMutation{
		SetHeaders: []*corev3.HeaderValueOption{
			setHeader(filterconfig.RequestIDHeader, s.requestID),
		},
		// 内部模型 Header 必须由请求体生成，不能信任客户端同名输入
		RemoveHeaders: []string{
			filterconfig.ModelHeader,
			filterconfig.UpstreamModelHeader,
		},
	}
	return headersResponse(requestMessage, mutation), nil
}

// handleEntryBody 在入口阶段提取客户端模型并保留未修改的请求体
// 后续每次上游尝试都从这份原文重新转换，避免重试使用前一厂商的请求格式
func (s *streamState) handleEntryBody(body *extprocv3.HttpBody) (*extprocv3.ProcessingResponse, error) {
	if !body.GetEndOfStream() {
		return nil, errRequestNotBuffered
	}
	// 入口只提取选路需要的信息，不提前转换为任何厂商协议
	metadata, err := chatcompletion.InspectRequest(body.GetBody())
	if err != nil {
		var invalid *chatcompletion.InvalidRequestError
		if errors.As(err, &invalid) {
			return invalidRequestResponse(invalid.Message()), nil
		}
		return nil, err
	}
	// 保存原文而不是转换结果，保证故障切换到不同协议的 Service 时可以重新生成请求
	s.request.setRequest(body.GetBody(), metadata)

	response := bodyResponse(requestMessage, nil, &extprocv3.HeaderMutation{
		SetHeaders: []*corev3.HeaderValueOption{setHeader(filterconfig.ModelHeader, metadata.Model)},
	})
	// Envoy 首次选路时看不到 JSON Body；写入内部模型 Header 后必须重新匹配模型线路
	response.GetRequestBody().GetResponse().ClearRouteCache = true
	// 动态元数据随访问日志输出，ALS 无需读取请求正文即可识别客户端模型
	response.DynamicMetadata = s.dynamicMetadata()
	return response, nil
}

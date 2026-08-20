package service

import (
	"strconv"
	"unicode/utf8"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
)

// messageSide 表示 ExtProc 响应对应 HTTP 请求侧还是响应侧
// 使用专用类型避免调用点出现无法直接理解的 true/false
type messageSide uint8

const (
	requestMessage messageSide = iota
	responseMessage
)

// headersResponse 构造请求侧或响应侧的 HeaderResponse
// ExtProc 协议要求每个收到的阶段消息都返回同类型响应，不能用一个通用消息代替
func headersResponse(side messageSide, mutation *extprocv3.HeaderMutation) *extprocv3.ProcessingResponse {
	common := &extprocv3.CommonResponse{
		Status:         extprocv3.CommonResponse_CONTINUE,
		HeaderMutation: mutation,
	}
	if side == requestMessage {
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_RequestHeaders{
				RequestHeaders: &extprocv3.HeadersResponse{Response: common},
			},
		}
	}
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseHeaders{
			ResponseHeaders: &extprocv3.HeadersResponse{Response: common},
		},
	}
}

// bodyResponse 构造请求侧或响应侧的 BodyResponse
func bodyResponse(
	side messageSide,
	bodyMutation *extprocv3.BodyMutation,
	headerMutation *extprocv3.HeaderMutation,
) *extprocv3.ProcessingResponse {
	common := &extprocv3.CommonResponse{
		Status:         extprocv3.CommonResponse_CONTINUE,
		BodyMutation:   bodyMutation,
		HeaderMutation: headerMutation,
	}
	if side == requestMessage {
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_RequestBody{
				RequestBody: &extprocv3.BodyResponse{Response: common},
			},
		}
	}
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseBody{
			ResponseBody: &extprocv3.BodyResponse{Response: common},
		},
	}
}

// replaceUpstreamRequest 在上游 filter 的 Header 阶段同时替换请求 Header 和 Body
// Envoy AI Gateway 也使用该模式，使每次重试都能从入口保存的原文重新生成厂商请求
func replaceUpstreamRequest(body []byte, headers *extprocv3.HeaderMutation) *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					Status:         extprocv3.CommonResponse_CONTINUE_AND_REPLACE,
					HeaderMutation: headers,
					BodyMutation: &extprocv3.BodyMutation{
						Mutation: &extprocv3.BodyMutation_Body{Body: body},
					},
				},
			},
		},
	}
}

// trailersResponse 构造请求侧或响应侧的 TrailerResponse
func trailersResponse(side messageSide) *extprocv3.ProcessingResponse {
	if side == requestMessage {
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_RequestTrailers{
				RequestTrailers: &extprocv3.TrailersResponse{},
			},
		}
	}
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseTrailers{
			ResponseTrailers: &extprocv3.TrailersResponse{},
		},
	}
}

func contentLengthMutation(length int) *extprocv3.HeaderMutation {
	// Body 被替换后必须同步长度，避免沿用模型服务响应或原请求的旧值
	return &extprocv3.HeaderMutation{
		SetHeaders: []*corev3.HeaderValueOption{setHeader("content-length", strconv.Itoa(length))},
	}
}

func setHeader(name, value string) *corev3.HeaderValueOption {
	return &corev3.HeaderValueOption{
		Header: &corev3.HeaderValue{
			Key:      name,
			RawValue: []byte(value),
		},
		// Envoy 建议新实现使用 raw_value，它既能保留任意字节，也避免依赖已废弃的 value 字段
		// 内部协议字段必须是单值，重复追加会导致 Envoy 或上游读取到歧义值
		AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
	}
}

func headerValue(headers *corev3.HeaderMap, name string) string {
	if headers == nil {
		return ""
	}
	for _, header := range headers.GetHeaders() {
		if header.GetKey() != name {
			continue
		}
		if header.GetValue() != "" {
			return header.GetValue()
		}
		if utf8.Valid(header.GetRawValue()) {
			return string(header.GetRawValue())
		}
		return ""
	}
	return ""
}

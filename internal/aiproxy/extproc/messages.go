package extproc

import (
	"strconv"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprochttpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
)

const (
	authorizationHeader    = "authorization"
	contentLengthHeader    = "content-length"
	contentEncodingHeader  = "content-encoding"
	contentTypeHeader      = "content-type"
	acceptEncodingHeader   = "accept-encoding"
	aiClusterHeader        = "x-ingate-ai-cluster-v1"
	anthropicAPIKeyHeader  = "x-api-key"
	anthropicVersionHeader = "anthropic-version"
	geminiAPIKeyHeader     = "x-goog-api-key"
	jsonContentType        = "application/json"
	sseContentType         = "text/event-stream"
)

var managedRequestHeaders = []string{
	authorizationHeader,
	anthropicAPIKeyHeader,
	anthropicVersionHeader,
	geminiAPIKeyHeader,
	aiClusterHeader,
	contentLengthHeader,
	contentEncodingHeader,
	contentTypeHeader,
	acceptEncodingHeader,
}

func headerValue(headers *corev3.HeaderMap, name string) string {
	if headers == nil {
		return ""
	}
	for _, header := range headers.Headers {
		if !strings.EqualFold(header.Key, name) {
			continue
		}
		if len(header.RawValue) > 0 {
			return string(header.RawValue)
		}
		return header.Value
	}
	return ""
}

func requestHeadersResponse() *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation: &extprocv3.HeaderMutation{RemoveHeaders: managedRequestHeaders},
				},
			},
		},
	}
}

func preparedRequestResponse(request preparedRequest) *extprocv3.ProcessingResponse {
	setHeaders := []*corev3.HeaderValueOption{
		headerValueOption(":path", request.path),
		headerValueOption(":authority", request.authority),
		headerValueOption(aiClusterHeader, request.cluster),
		headerValueOption(contentTypeHeader, jsonContentType),
		headerValueOption(contentLengthHeader, strconv.Itoa(len(request.body))),
	}
	for _, header := range request.headers {
		setHeaders = append(setHeaders, headerValueOption(header.Name, header.Value))
	}
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestBody{
			RequestBody: &extprocv3.BodyResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation: &extprocv3.HeaderMutation{SetHeaders: setHeaders},
					BodyMutation:   bodyMutation(request.body),
					// 模型名称来自请求体，写入目标 Cluster 后必须让 Router 重新计算路由动作
					ClearRouteCache: true,
				},
			},
		},
	}
}

func responseHeadersResponse(
	contentType string,
	bodyMode extprochttpv3.ProcessingMode_BodySendMode,
) *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseHeaders{
			ResponseHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation: responseHeaderMutation(0, contentType),
				},
			},
		},
		ModeOverride: processingMode(bodyMode),
	}
}

func replaceResponseHeaders(
	contentType string,
	body []byte,
	statusCode int,
) *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseHeaders{
			ResponseHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					Status:         extprocv3.CommonResponse_CONTINUE_AND_REPLACE,
					HeaderMutation: responseHeaderMutation(statusCode, contentType),
					BodyMutation:   bodyMutation(body),
				},
			},
		},
		ModeOverride: processingMode(extprochttpv3.ProcessingMode_NONE),
	}
}

func replaceResponseBody(statusCode int, body []byte) *extprocv3.ProcessingResponse {
	response := &extprocv3.CommonResponse{BodyMutation: bodyMutation(body)}
	if statusCode != 0 {
		response.HeaderMutation = responseHeaderMutation(statusCode, jsonContentType)
	}
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ResponseBody{
			ResponseBody: &extprocv3.BodyResponse{Response: response},
		},
	}
}

func processingMode(responseBodyMode extprochttpv3.ProcessingMode_BodySendMode) *extprochttpv3.ProcessingMode {
	return &extprochttpv3.ProcessingMode{
		RequestHeaderMode:   extprochttpv3.ProcessingMode_SEND,
		ResponseHeaderMode:  extprochttpv3.ProcessingMode_SEND,
		RequestBodyMode:     extprochttpv3.ProcessingMode_BUFFERED,
		ResponseBodyMode:    responseBodyMode,
		RequestTrailerMode:  extprochttpv3.ProcessingMode_SKIP,
		ResponseTrailerMode: extprochttpv3.ProcessingMode_SKIP,
	}
}

func responseHeaderMutation(statusCode int, contentType string) *extprocv3.HeaderMutation {
	setHeaders := []*corev3.HeaderValueOption{headerValueOption(contentTypeHeader, contentType)}
	if statusCode != 0 {
		setHeaders = append(setHeaders, headerValueOption(":status", strconv.Itoa(statusCode)))
	}
	return &extprocv3.HeaderMutation{
		SetHeaders:    setHeaders,
		RemoveHeaders: []string{contentLengthHeader, contentEncodingHeader},
	}
}

func immediateResponse(response localResponse) *extprocv3.ProcessingResponse {
	setHeaders := make([]*corev3.HeaderValueOption, 0, len(response.headers))
	for _, header := range response.headers {
		setHeaders = append(setHeaders, headerValueOption(header[0], header[1]))
	}
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ImmediateResponse{
			ImmediateResponse: &extprocv3.ImmediateResponse{
				Status:  &typev3.HttpStatus{Code: typev3.StatusCode(response.statusCode)},
				Headers: &extprocv3.HeaderMutation{SetHeaders: setHeaders},
				Body:    response.body,
				Details: "ingate_ai_proxy_rejected",
			},
		},
	}
}

func unauthorizedLocalResponse() localResponse {
	response := rejectionResponse(401, "Invalid or missing API key", "invalid_request_error", "invalid_api_key", nil)
	response.headers = append(response.headers, [2]string{"www-authenticate", "Bearer"})
	return response
}

func authenticationUnavailableLocalResponse() localResponse {
	return rejectionResponse(
		503,
		"API key authentication is temporarily unavailable",
		"server_error",
		"authentication_unavailable",
		nil,
	)
}

func bodyMutation(body []byte) *extprocv3.BodyMutation {
	if len(body) == 0 {
		return &extprocv3.BodyMutation{Mutation: &extprocv3.BodyMutation_ClearBody{ClearBody: true}}
	}
	return &extprocv3.BodyMutation{Mutation: &extprocv3.BodyMutation_Body{Body: body}}
}

func headerValueOption(name, value string) *corev3.HeaderValueOption {
	return &corev3.HeaderValueOption{
		Header: &corev3.HeaderValue{
			Key:      name,
			RawValue: []byte(value),
		},
		AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
	}
}

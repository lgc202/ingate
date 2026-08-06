// Package extproc 实现 Envoy AI 请求的外部处理协议
package extproc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprochttpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/lgc202/ingate/internal/pkg/aiproxyconfig"
	"github.com/lgc202/ingate/internal/pkg/bearer"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	grpcmetadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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

// Server 实现 Envoy External Processing gRPC 服务
type Server struct {
	extprocv3.UnimplementedExternalProcessorServer

	authenticator *authenticator
	logger        *slog.Logger
}

type requestState struct {
	config            aiproxyconfig.Config
	configErr         error
	proxy             *modelProxy
	grant             grant
	responseTransform responseTransform
	responseStatus    int
	responseStream    responseStream
	requestPrepared   bool
	responseStarted   bool
	responseClosed    bool
}

var _ extprocv3.ExternalProcessorServer = (*Server)(nil)

// NewServer 创建 AI 请求外部处理服务
func NewServer(redisClient *redis.Client, logger *slog.Logger) *Server {
	return &Server{
		authenticator: newAuthenticator(redisClient),
		logger:        logger,
	}
}

// Process 在单条 ExtProc 流中完成认证、模型选择、协议转换和响应归一化
func (s *Server) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	config, configErr := decodeRouteConfig(stream.Context())
	state := &requestState{config: config, configErr: configErr}
	for {
		request, err := stream.Recv()
		if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
			return nil
		}
		if err != nil {
			return fmt.Errorf("receive ExtProc request: %w", err)
		}

		response, terminal, err := s.processMessage(stream.Context(), state, request)
		if err != nil {
			return err
		}
		if err := stream.Send(response); err != nil {
			return fmt.Errorf("send ExtProc response: %w", err)
		}
		if terminal {
			return nil
		}
	}
}

func (s *Server) processMessage(
	ctx context.Context,
	state *requestState,
	request *extprocv3.ProcessingRequest,
) (*extprocv3.ProcessingResponse, bool, error) {
	switch message := request.Request.(type) {
	case *extprocv3.ProcessingRequest_RequestHeaders:
		if state.proxy != nil {
			return nil, false, status.Error(codes.InvalidArgument, "request headers were received more than once")
		}
		return s.processRequestHeaders(ctx, state, message.RequestHeaders)
	case *extprocv3.ProcessingRequest_RequestBody:
		if state.proxy == nil || state.requestPrepared {
			return nil, false, status.Error(codes.InvalidArgument, "request body was received out of order")
		}
		return s.processRequestBody(state, message.RequestBody)
	case *extprocv3.ProcessingRequest_ResponseHeaders:
		if !state.requestPrepared || state.responseStarted {
			return nil, false, status.Error(codes.InvalidArgument, "response headers were received out of order")
		}
		return s.processResponseHeaders(state, message.ResponseHeaders)
	case *extprocv3.ProcessingRequest_ResponseBody:
		if !state.responseStarted {
			return nil, false, status.Error(codes.InvalidArgument, "response body was received before response headers")
		}
		return s.processResponseBody(state, message.ResponseBody)
	default:
		return nil, false, status.Errorf(codes.InvalidArgument, "unsupported ExtProc request message %T", request.Request)
	}
}

func (s *Server) processRequestHeaders(
	ctx context.Context,
	state *requestState,
	headers *extprocv3.HttpHeaders,
) (*extprocv3.ProcessingResponse, bool, error) {
	if state.configErr != nil {
		s.logger.Error("decode AI route config failed", "err", state.configErr)
		return immediateResponse(internalErrorResponse()), true, nil
	}
	state.proxy = newModelProxy(state.config)
	if response := state.proxy.validateEndpoint(
		headerValue(headers.GetHeaders(), ":method"),
		headerValue(headers.GetHeaders(), ":path"),
	); response != nil {
		return immediateResponse(*response), true, nil
	}

	secret, ok := bearerSecret(headerValue(headers.GetHeaders(), authorizationHeader))
	if !ok {
		return immediateResponse(unauthorizedLocalResponse()), true, nil
	}
	currentGrant, authorized, err := s.authenticator.authenticate(ctx, secret)
	if err != nil {
		s.logger.Error("authenticate AI access key failed", "err", err)
		return immediateResponse(authenticationUnavailableLocalResponse()), true, nil
	}
	if !authorized {
		return immediateResponse(unauthorizedLocalResponse()), true, nil
	}
	if headers.GetEndOfStream() {
		response := rejectionResponse(400, "Request body is required", "invalid_request_error", "invalid_request", nil)
		return immediateResponse(response), true, nil
	}
	state.grant = currentGrant
	return requestHeadersResponse(), false, nil
}

func (s *Server) processRequestBody(
	state *requestState,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, bool, error) {
	if len(body.GetBody()) > aiproxyconfig.MaxRequestBodyBytes {
		response := rejectionResponse(413, "Request body is too large", "invalid_request_error", "request_too_large", nil)
		return immediateResponse(response), true, nil
	}
	prepared, response := state.proxy.prepareRequest(body.GetBody(), state.grant.allows)
	if response != nil {
		return immediateResponse(*response), true, nil
	}
	state.requestPrepared = true
	state.responseTransform = prepared.response
	return preparedRequestResponse(prepared), false, nil
}

func (s *Server) processResponseHeaders(
	state *requestState,
	headers *extprocv3.HttpHeaders,
) (*extprocv3.ProcessingResponse, bool, error) {
	state.responseStarted = true
	statusCode, err := strconv.Atoi(headerValue(headers.GetHeaders(), ":status"))
	if err != nil {
		s.logger.Error("parse AI upstream response status failed", "err", err)
		statusCode = 502
	}
	state.responseStatus = statusCode

	streaming := state.responseTransform.stream && statusCode < 400
	contentType := jsonContentType
	bodyMode := extprochttpv3.ProcessingMode_BUFFERED
	if streaming {
		contentType = sseContentType
		bodyMode = extprochttpv3.ProcessingMode_STREAMED
		state.responseStream, err = state.proxy.newResponseStream(state.responseTransform)
		if err != nil {
			s.logger.Error("create AI response stream failed", "err", err)
			return replaceResponseHeaders(jsonContentType, responseErrorBody(), 502), true, nil
		}
	}

	if !headers.GetEndOfStream() {
		return responseHeadersResponse(contentType, bodyMode), false, nil
	}
	if streaming {
		body, finishErr := state.responseStream.Finish()
		if finishErr != nil {
			s.logger.Error("finish empty AI response stream failed", "err", finishErr)
			body = streamErrorBody()
		}
		return replaceResponseHeaders(contentType, body, 0), true, nil
	}
	body, transformErr := state.proxy.transformResponse(state.responseTransform, statusCode, nil)
	if transformErr != nil {
		s.logger.Error("transform empty AI response failed", "err", transformErr)
		return replaceResponseHeaders(jsonContentType, responseErrorBody(), 502), true, nil
	}
	return replaceResponseHeaders(contentType, body, 0), true, nil
}

func (s *Server) processResponseBody(
	state *requestState,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, bool, error) {
	if state.responseTransform.stream && state.responseStatus < 400 {
		return s.processStreamingResponse(state, body)
	}
	if len(body.GetBody()) > aiproxyconfig.MaxResponseBodyBytes {
		s.logger.Error("AI upstream response body is too large", "bytes", len(body.GetBody()))
		return replaceResponseBody(502, responseErrorBody()), body.GetEndOfStream(), nil
	}
	transformed, err := state.proxy.transformResponse(state.responseTransform, state.responseStatus, body.GetBody())
	if err != nil {
		s.logger.Error("transform AI upstream response failed", "err", err)
		return replaceResponseBody(502, responseErrorBody()), body.GetEndOfStream(), nil
	}
	return replaceResponseBody(0, transformed), body.GetEndOfStream(), nil
}

func (s *Server) processStreamingResponse(
	state *requestState,
	body *extprocv3.HttpBody,
) (*extprocv3.ProcessingResponse, bool, error) {
	if state.responseClosed {
		return replaceResponseBody(0, nil), body.GetEndOfStream(), nil
	}
	transformed, err := state.responseStream.Push(body.GetBody())
	if err != nil {
		s.logger.Error("transform AI upstream stream failed", "err", err)
		state.responseClosed = true
		return replaceResponseBody(0, streamErrorBody()), body.GetEndOfStream(), nil
	}
	if body.GetEndOfStream() {
		tail, finishErr := state.responseStream.Finish()
		if finishErr != nil {
			s.logger.Error("finish AI upstream stream failed", "err", finishErr)
			tail = streamErrorBody()
		}
		transformed = append(transformed, tail...)
	}
	return replaceResponseBody(0, transformed), body.GetEndOfStream(), nil
}

func decodeRouteConfig(ctx context.Context) (aiproxyconfig.Config, error) {
	metadata, ok := grpcmetadata.FromIncomingContext(ctx)
	if !ok {
		return aiproxyconfig.Config{}, errors.New("ExtProc gRPC metadata is missing")
	}
	values := metadata.Get(aiproxyconfig.GRPCMetadataKey)
	if len(values) != 1 {
		return aiproxyconfig.Config{}, fmt.Errorf("ExtProc gRPC metadata %q must contain one value", aiproxyconfig.GRPCMetadataKey)
	}
	return aiproxyconfig.Decode(values[0])
}

func bearerSecret(authorization string) (string, bool) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || !bearer.ValidToken(parts[1]) {
		return "", false
	}
	return parts[1], true
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

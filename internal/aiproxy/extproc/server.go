// Package extproc 实现 Envoy AI 请求的外部处理协议
package extproc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lgc202/ingate/internal/aiproxy/accesskey"
	"github.com/lgc202/ingate/internal/pkg/aiproxyconfig"
)

// Server 实现 Envoy External Processing gRPC 服务
type Server struct {
	extprocv3.UnimplementedExternalProcessorServer

	authenticator *accesskey.Authenticator
	logger        *slog.Logger
}

// requestState 只在一条 ExtProc 双向流内使用，记录请求各阶段之间必须传递的状态
type requestState struct {
	config            aiproxyconfig.Config
	configErr         error
	proxy             *modelProxy
	grant             accesskey.Grant
	responseTransform responseTransform
	responseStatus    int
	responseStream    responseStream
	requestPrepared   bool
	responseStarted   bool
	responseClosed    bool
}

var _ extprocv3.ExternalProcessorServer = (*Server)(nil)

// NewServer 创建 AI 请求外部处理服务
func NewServer(authenticator *accesskey.Authenticator, logger *slog.Logger) *Server {
	return &Server{
		authenticator: authenticator,
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

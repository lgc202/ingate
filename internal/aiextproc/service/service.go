// Package service 实现 Envoy External Processing 协议
package service

import (
	"errors"
	"io"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Service 接收 Envoy 发来的请求和响应阶段消息
type Service struct {
	extprocv3.UnimplementedExternalProcessorServer
}

// NewService 创建 External Processing 服务
func NewService() *Service {
	return &Service{}
}

// Process 按 Envoy External Processing 协议处理一条 HTTP 请求双向流
func (*Service) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	for {
		request, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if request.GetObservabilityMode() {
			// Observability Mode 明确禁止等待响应，返回消息会被 Envoy 忽略
			continue
		}

		response := continueResponse(request)
		if response == nil {
			return status.Error(codes.InvalidArgument, "processing request does not contain a supported phase")
		}
		if err := stream.Send(response); err != nil {
			return err
		}
	}
}

// continueResponse 返回与请求阶段严格对应的透明响应
// ExtProc 在非 Observability Mode 下要求每条阶段消息都有且只有一个同类型响应
func continueResponse(request *extprocv3.ProcessingRequest) *extprocv3.ProcessingResponse {
	common := &extprocv3.CommonResponse{Status: extprocv3.CommonResponse_CONTINUE}
	switch request.GetRequest().(type) {
	case *extprocv3.ProcessingRequest_RequestHeaders:
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_RequestHeaders{
				RequestHeaders: &extprocv3.HeadersResponse{Response: common},
			},
		}
	case *extprocv3.ProcessingRequest_ResponseHeaders:
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_ResponseHeaders{
				ResponseHeaders: &extprocv3.HeadersResponse{Response: common},
			},
		}
	case *extprocv3.ProcessingRequest_RequestBody:
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_RequestBody{
				RequestBody: &extprocv3.BodyResponse{Response: common},
			},
		}
	case *extprocv3.ProcessingRequest_ResponseBody:
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_ResponseBody{
				ResponseBody: &extprocv3.BodyResponse{Response: common},
			},
		}
	case *extprocv3.ProcessingRequest_RequestTrailers:
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_RequestTrailers{
				RequestTrailers: &extprocv3.TrailersResponse{},
			},
		}
	case *extprocv3.ProcessingRequest_ResponseTrailers:
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_ResponseTrailers{
				ResponseTrailers: &extprocv3.TrailersResponse{},
			},
		}
	default:
		return nil
	}
}

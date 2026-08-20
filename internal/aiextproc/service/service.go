// Package service 实现 Envoy External Processing 协议
package service

import (
	"errors"
	"fmt"
	"io"
	"sync"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lgc202/ingate/internal/aiextproc/modelservice"
)

// Service 接收 Envoy 发来的入口和上游 External Processing 流
// requests 只在单次 HTTP 请求存活期间关联两类流，不承担持久化或跨实例共享
type Service struct {
	extprocv3.UnimplementedExternalProcessorServer

	mu       sync.RWMutex
	requests map[string]*requestState
	models   *modelservice.Cache
}

// NewService 创建 External Processing 服务
func NewService(models *modelservice.Cache) *Service {
	return &Service{
		requests: make(map[string]*requestState),
		models:   models,
	}
}

// Process 按 Envoy External Processing 协议处理一条双向流
// Envoy 会分别为入口 filter 和每次上游尝试创建流，二者不是同一个 gRPC stream
func (s *Service) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	state := streamState{service: s}
	defer state.close()

	for {
		request, err := stream.Recv()
		if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
			return nil
		}
		if err != nil {
			return fmt.Errorf("receive ExtProc request: %w", err)
		}
		if request.GetObservabilityMode() {
			// Observability Mode 明确禁止等待响应，返回消息会被 Envoy 忽略
			continue
		}

		response, err := state.handle(request)
		if err != nil {
			return processingStatus(err)
		}
		if err := stream.Send(response); err != nil {
			return fmt.Errorf("send ExtProc response: %w", err)
		}
	}
}

func (s *Service) registerRequest() (string, *requestState) {
	// 关联 ID 由服务端生成并覆盖同名客户端 Header，避免不同请求串用共享状态
	id := uuid.NewString()
	request := &requestState{}
	s.mu.Lock()
	s.requests[id] = request
	s.mu.Unlock()
	return id, request
}

func (s *Service) findRequest(id string) (*requestState, bool) {
	s.mu.RLock()
	request, ok := s.requests[id]
	s.mu.RUnlock()
	return request, ok
}

func (s *Service) deleteRequest(id string) {
	// 入口流覆盖完整请求生命周期，它结束后不会再产生新的上游尝试
	s.mu.Lock()
	delete(s.requests, id)
	s.mu.Unlock()
}

func processingStatus(err error) error {
	// ExtProc 配置错误与请求协议错误使用不同状态，便于 Envoy 日志定位部署问题
	switch {
	case errors.Is(err, errUnsupportedPhase):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, errRequestNotBuffered), errors.Is(err, errResponseNotBuffered):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Errorf(codes.Internal, "process HTTP exchange: %v", err)
	}
}

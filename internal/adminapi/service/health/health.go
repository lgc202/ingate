// Package health 实现进程健康检查 API
package health

import (
	"context"

	"github.com/go-kratos/kratos/v3/transport"
	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// Service 实现进程存活检查
type Service struct{}

// NewService 创建健康检查协议服务
func NewService() *Service {
	return &Service{}
}

func (s *Service) Check(ctx context.Context, _ *emptypb.Empty) (*adminv1.HealthReply, error) {
	var id string
	if tr, ok := transport.FromServerContext(ctx); ok {
		id = tr.RequestHeader().Get(requestid.Header)
	}
	return &adminv1.HealthReply{Status: "ok", RequestId: id}, nil
}

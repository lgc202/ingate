package service

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/go-kratos/kratos/v3/transport"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// HealthService 实现进程存活检查
type HealthService struct{}

// NewHealthService 创建健康检查协议服务
func NewHealthService() *HealthService {
	return &HealthService{}
}

func (s *HealthService) Check(ctx context.Context, _ *emptypb.Empty) (*adminv1.HealthReply, error) {
	var id string
	if tr, ok := transport.FromServerContext(ctx); ok {
		id = tr.RequestHeader().Get(requestid.Header)
	}
	return &adminv1.HealthReply{Status: "ok", RequestId: id}, nil
}

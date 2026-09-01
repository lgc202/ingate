// Package system 实现运维助手的进程状态协议。
package system

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	systembiz "github.com/lgc202/ingate/internal/assistant/biz/system"
)

// Service 将系统状态用例适配为 Assistant API。
type Service struct {
	usecase *systembiz.Usecase
}

// NewService 创建系统状态协议服务。
func NewService(usecase *systembiz.Usecase) *Service {
	return &Service{usecase: usecase}
}

// GetReadiness 返回进程及每个必需组件的当前就绪状态。
func (s *Service) GetReadiness(ctx context.Context, _ *emptypb.Empty) (*assistantv1.Readiness, error) {
	report := s.usecase.Check(ctx)
	components := make([]*assistantv1.ComponentReadiness, 0, len(report.Components))
	for _, component := range report.Components {
		components = append(components, &assistantv1.ComponentReadiness{
			Name:   component.Name,
			Status: readinessStatus(component.Status),
		})
	}
	return &assistantv1.Readiness{
		Status:     readinessStatus(report.Status),
		Components: components,
	}, nil
}

func readinessStatus(status systembiz.Status) assistantv1.ReadinessStatus {
	if status == systembiz.StatusReady {
		return assistantv1.ReadinessStatus_READINESS_STATUS_READY
	}
	return assistantv1.ReadinessStatus_READINESS_STATUS_UNAVAILABLE
}

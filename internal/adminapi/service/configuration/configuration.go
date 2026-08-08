// Package configuration 实现配置发布状态查询 API
package configuration

import (
	"context"

	"github.com/google/wire"
	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	configurationbiz "github.com/lgc202/ingate/internal/adminapi/biz/configuration"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// ProviderSet 提供配置发布状态协议服务
var ProviderSet = wire.NewSet(NewService)

// Service 实现配置生效状态查询 API
type Service struct {
	usecase *configurationbiz.Usecase
}

// NewService 创建配置状态协议服务
func NewService(usecase *configurationbiz.Usecase) *Service {
	return &Service{usecase: usecase}
}

func (s *Service) GetConfigurationStatus(ctx context.Context, _ *emptypb.Empty) (*adminv1.ConfigurationStatusReply, error) {
	report, err := s.usecase.Get(ctx)
	if err != nil {
		return nil, adminservice.OperationError(err, "查询配置状态失败")
	}
	reply := &adminv1.ConfigurationStatusReply{
		Summary: &adminv1.ConfigurationSummary{
			Total: int32(report.Summary.Total), Ready: int32(report.Summary.Ready),
			Pending: int32(report.Summary.Pending), Error: int32(report.Summary.Error),
			Disabled: int32(report.Summary.Disabled),
		},
		Items: make([]*adminv1.ConfigurationItem, 0, len(report.Items)),
	}
	for _, item := range report.Items {
		reply.Items = append(reply.Items, &adminv1.ConfigurationItem{
			Kind: string(item.Kind), Id: item.ID, Name: item.Name, Status: adminservice.ResourceStatus(item.Status),
		})
	}
	return reply, nil
}

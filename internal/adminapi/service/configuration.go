package service

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
)

// ConfigurationService 实现配置生效状态查询 API
type ConfigurationService struct {
	usecase *biz.ConfigurationUsecase
}

// NewConfigurationService 创建配置状态协议服务
func NewConfigurationService(usecase *biz.ConfigurationUsecase) *ConfigurationService {
	return &ConfigurationService{usecase: usecase}
}

func (s *ConfigurationService) GetConfigurationStatus(ctx context.Context, _ *emptypb.Empty) (*adminv1.ConfigurationStatusReply, error) {
	report, err := s.usecase.Get(ctx)
	if err != nil {
		return nil, operationError(err, "查询配置状态失败")
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
			Kind: string(item.Kind), Id: item.ID, Name: item.Name, Status: resourceStatus(item.Status),
		})
	}
	return reply, nil
}

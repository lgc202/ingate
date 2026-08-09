// Package configuration 实现配置发布状态查询 API
package configuration

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	configurationbiz "github.com/lgc202/ingate/internal/adminapi/biz/configuration"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现配置生效状态查询 API
type Service struct {
	usecase *configurationbiz.Usecase
}

// NewService 创建配置状态协议服务
func NewService(usecase *configurationbiz.Usecase) *Service {
	return &Service{usecase: usecase}
}

func (s *Service) GetConfigurationSummary(ctx context.Context, _ *emptypb.Empty) (*adminv1.ConfigurationSummary, error) {
	summary, err := s.usecase.GetSummary(ctx)
	if err != nil {
		return nil, err
	}
	return &adminv1.ConfigurationSummary{
		Total: int32(summary.Total), Ready: int32(summary.Ready),
		Pending: int32(summary.Pending), Error: int32(summary.Error),
		Disabled: int32(summary.Disabled),
	}, nil
}

func (s *Service) ListConfigurationItems(
	ctx context.Context,
	request *adminv1.ListRequest,
) (*adminv1.ListConfigurationItemsReply, error) {
	result, err := s.usecase.ListItems(ctx, adminservice.PageRequest(request))
	if err != nil {
		return nil, err
	}
	reply := &adminv1.ListConfigurationItemsReply{
		Items: make([]*adminv1.ConfigurationItem, 0, len(result.Items)),
		Page:  adminservice.PageInfo(result.NextToken),
	}
	for _, item := range result.Items {
		reply.Items = append(reply.Items, &adminv1.ConfigurationItem{
			Kind: string(item.Kind), Id: item.ID, Name: item.Name, Status: adminservice.NewResourceStatus(item.Status),
		})
	}
	return reply, nil
}

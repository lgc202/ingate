// Package gateway 提供 Gateway 管理 API。
package gateway

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	gatewaybiz "github.com/lgc202/ingate/internal/adminapi/biz/gateway"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
)

// Service 实现 Gateway 管理 API。
type Service struct {
	gateways *gatewaybiz.Usecase
}

// NewService 创建 Gateway 协议服务。
func NewService(gateways *gatewaybiz.Usecase) *Service {
	return &Service{gateways: gateways}
}

// ListGateways 返回满足筛选条件的 Gateway 列表。
func (s *Service) ListGateways(
	ctx context.Context,
	request *adminv1.ListGatewaysRequest,
) (*adminv1.ListGatewaysResponse, error) {
	page, err := s.gateways.List(
		ctx,
		adminservice.PageRequest(request.GetLimit(), request.GetCursor()),
		adminservice.ResourceFilter(request.GetQuery(), request.Enabled, request.GetState()),
	)
	if err != nil {
		return nil, err
	}
	gateways := make([]*adminv1.Gateway, len(page.Items))
	for i := range page.Items {
		gateways[i] = gatewayResponse(&page.Items[i])
	}
	return &adminv1.ListGatewaysResponse{
		Gateways:   gateways,
		NextCursor: page.NextCursor,
	}, nil
}

// GetGateway 返回指定 Gateway。
func (s *Service) GetGateway(
	ctx context.Context,
	request *adminv1.GetGatewayRequest,
) (*adminv1.Gateway, error) {
	gateway, err := s.gateways.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return gatewayResponse(gateway), nil
}

// CreateGateway 创建 Gateway。
func (s *Service) CreateGateway(
	ctx context.Context,
	request *adminv1.CreateGatewayRequest,
) (*adminv1.Gateway, error) {
	spec, err := parseGatewaySpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetListeners(),
	)
	if err != nil {
		return nil, err
	}
	gateway, err := s.gateways.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return gatewayResponse(gateway), nil
}

// UpdateGateway 完整替换 Gateway 配置。
func (s *Service) UpdateGateway(
	ctx context.Context,
	request *adminv1.UpdateGatewayRequest,
) (*adminv1.Gateway, error) {
	spec, err := parseGatewaySpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetListeners(),
	)
	if err != nil {
		return nil, err
	}
	gateway, err := s.gateways.Replace(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return gatewayResponse(gateway), nil
}

// DeleteGateway 删除 Gateway。
func (s *Service) DeleteGateway(
	ctx context.Context,
	request *adminv1.DeleteGatewayRequest,
) (*emptypb.Empty, error) {
	if err := s.gateways.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

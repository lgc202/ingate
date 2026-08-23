// Package gateway 提供 Gateway 管理 API
package gateway

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	gatewaybiz "github.com/lgc202/ingate/internal/adminapi/biz/gateway"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现网关入口管理 API
type Service struct {
	gateways *gatewaybiz.Service
}

// NewService 创建网关入口协议服务
func NewService(gateways *gatewaybiz.Service) *Service {
	return &Service{gateways: gateways}
}

func (s *Service) ListGateways(ctx context.Context, request *adminv1.ListGatewaysRequest) (*adminv1.ListGatewaysResponse, error) {
	page, err := s.gateways.List(
		ctx,
		adminservice.PageRequest(request.GetLimit(), request.GetCursor()),
		adminservice.ResourceFilter(request.GetQuery(), request.Enabled, request.GetState()),
	)
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListGatewaysResponse{
		Gateways:   make([]*adminv1.Gateway, 0, len(page.Items)),
		NextCursor: page.NextCursor,
	}
	for i := range page.Items {
		response.Gateways = append(response.Gateways, gatewayResponse(&page.Items[i]))
	}
	return response, nil
}

func (s *Service) GetGateway(ctx context.Context, request *adminv1.GetGatewayRequest) (*adminv1.Gateway, error) {
	gateway, err := s.gateways.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return gatewayResponse(gateway), nil
}

func (s *Service) CreateGateway(ctx context.Context, request *adminv1.CreateGatewayRequest) (*adminv1.Gateway, error) {
	spec, err := createSpec(request)
	if err != nil {
		return nil, err
	}
	gateway, err := s.gateways.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return gatewayResponse(gateway), nil
}

func (s *Service) UpdateGateway(ctx context.Context, request *adminv1.UpdateGatewayRequest) (*adminv1.Gateway, error) {
	spec, err := updateSpec(request)
	if err != nil {
		return nil, err
	}
	gateway, err := s.gateways.Update(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return gatewayResponse(gateway), nil
}

func (s *Service) DeleteGateway(ctx context.Context, request *adminv1.DeleteGatewayRequest) (*emptypb.Empty, error) {
	if err := s.gateways.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

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
	business *gatewaybiz.Service
}

// NewService 创建网关入口协议服务
func NewService(business *gatewaybiz.Service) *Service {
	return &Service{business: business}
}

func (s *Service) ListGateways(ctx context.Context, request *adminv1.ListGatewaysRequest) (*adminv1.ListGatewaysResponse, error) {
	result, err := s.business.List(ctx, adminservice.PageRequest(request.GetLimit(), request.GetCursor()))
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListGatewaysResponse{
		Gateways:   make([]*adminv1.Gateway, 0, len(result.Items)),
		NextCursor: result.NextCursor,
	}
	for i := range result.Items {
		response.Gateways = append(response.Gateways, gatewayFromResource(&result.Items[i]))
	}
	return response, nil
}

func (s *Service) GetGateway(ctx context.Context, request *adminv1.GetGatewayRequest) (*adminv1.Gateway, error) {
	item, err := s.business.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return gatewayFromResource(item), nil
}

func (s *Service) CreateGateway(ctx context.Context, request *adminv1.CreateGatewayRequest) (*adminv1.Gateway, error) {
	spec, err := buildGatewaySpec(request.GetName(), request.GetEnabled(), request.GetListeners())
	if err != nil {
		return nil, err
	}
	item, err := s.business.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return gatewayFromResource(item), nil
}

func (s *Service) UpdateGateway(ctx context.Context, request *adminv1.UpdateGatewayRequest) (*adminv1.Gateway, error) {
	spec, err := buildGatewaySpec(request.GetName(), request.GetEnabled(), request.GetListeners())
	if err != nil {
		return nil, err
	}
	item, err := s.business.Update(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return gatewayFromResource(item), nil
}

func (s *Service) DeleteGateway(ctx context.Context, request *adminv1.DeleteGatewayRequest) (*emptypb.Empty, error) {
	if err := s.business.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

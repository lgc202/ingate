// Package gateway 实现 Gateway 管理 API
package gateway

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	gatewaybiz "github.com/lgc202/ingate/internal/adminapi/biz/gateway"
)

// Service 实现网关入口管理 API
type Service struct {
	usecase *gatewaybiz.Usecase
}

// NewService 创建网关入口协议服务
func NewService(usecase *gatewaybiz.Usecase) *Service {
	return &Service{usecase: usecase}
}

func (s *Service) ListGateways(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListGatewaysReply, error) {
	items, err := s.usecase.List(ctx)
	if err != nil {
		return nil, err
	}
	reply := &adminv1.ListGatewaysReply{Gateways: make([]*adminv1.Gateway, 0, len(items))}
	for i := range items {
		reply.Gateways = append(reply.Gateways, newGatewayReply(&items[i]))
	}
	return reply, nil
}

func (s *Service) GetGateway(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.GetGatewayReply, error) {
	item, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return &adminv1.GetGatewayReply{Gateway: newGatewayReply(item)}, nil
}

func (s *Service) CreateGateway(ctx context.Context, request *adminv1.CreateGatewayRequest) (*adminv1.MutationReply, error) {
	spec, err := buildGatewaySpec(request.GetName(), request.GetDescription(), request.GetListeners(), request.GetHostnames())
	if err != nil {
		return nil, err
	}
	id, err := s.usecase.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: id}, nil
}

func (s *Service) UpdateGateway(ctx context.Context, request *adminv1.UpdateGatewayRequest) (*adminv1.MutationReply, error) {
	spec, err := buildGatewaySpec(request.GetName(), request.GetDescription(), request.GetListeners(), request.GetHostnames())
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) SetGatewayEnabled(ctx context.Context, request *adminv1.SetEnabledRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.SetEnabled(ctx, request.GetId(), request.GetEnabled()); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) DeleteGateway(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

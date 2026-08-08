// Package route 实现 Route 管理 API
package route

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	routebiz "github.com/lgc202/ingate/internal/adminapi/biz/route"
)

// Service 实现路由规则管理 API
type Service struct {
	usecase *routebiz.Usecase
}

// NewService 创建路由协议服务
func NewService(usecase *routebiz.Usecase) *Service {
	return &Service{usecase: usecase}
}

func (s *Service) ListRoutes(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListRoutesReply, error) {
	items, err := s.usecase.List(ctx)
	if err != nil {
		return nil, err
	}
	reply := &adminv1.ListRoutesReply{Routes: make([]*adminv1.Route, 0, len(items))}
	for i := range items {
		reply.Routes = append(reply.Routes, newRouteReply(&items[i]))
	}
	return reply, nil
}

func (s *Service) GetRoute(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.Route, error) {
	item, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return newRouteReply(item), nil
}

func (s *Service) CreateRoute(ctx context.Context, request *adminv1.CreateRouteRequest) (*adminv1.MutationReply, error) {
	spec, err := buildRouteSpec(request.GetName(), request.GetGatewayIds(), request.GetHostnames(), request.Enabled, request.GetRules())
	if err != nil {
		return nil, err
	}
	id, err := s.usecase.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: id}, nil
}

func (s *Service) UpdateRoute(ctx context.Context, request *adminv1.UpdateRouteRequest) (*adminv1.MutationReply, error) {
	spec, err := buildRouteSpec(request.GetName(), request.GetGatewayIds(), request.GetHostnames(), request.Enabled, request.GetRules())
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) SetRouteEnabled(ctx context.Context, request *adminv1.SetEnabledRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.SetEnabled(ctx, request.GetId(), request.GetEnabled()); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) DeleteRoute(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

// Package route 实现 Route 管理 API
package route

import (
	"context"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	routebiz "github.com/lgc202/ingate/internal/adminapi/biz/route"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现路由规则管理 API
type Service struct {
	usecase *routebiz.Usecase
}

// NewService 创建路由协议服务
func NewService(usecase *routebiz.Usecase) *Service {
	return &Service{usecase: usecase}
}

func (s *Service) ListRoutes(ctx context.Context, request *adminv1.ListRequest) (*adminv1.ListRoutesReply, error) {
	result, err := s.usecase.List(ctx, adminservice.PageRequest(request))
	if err != nil {
		return nil, err
	}
	reply := &adminv1.ListRoutesReply{Routes: make([]*adminv1.Route, 0, len(result.Items)), Page: adminservice.PageInfo(result.NextToken)}
	for i := range result.Items {
		reply.Routes = append(reply.Routes, newRouteReply(&result.Items[i]))
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
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec, request.Enabled != nil); err != nil {
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

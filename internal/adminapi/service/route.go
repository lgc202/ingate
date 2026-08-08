package service

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
)

// RouteService 实现路由规则管理 API
type RouteService struct {
	usecase *biz.RouteUsecase
}

// NewRouteService 创建路由协议服务
func NewRouteService(usecase *biz.RouteUsecase) *RouteService {
	return &RouteService{usecase: usecase}
}

func (s *RouteService) ListRoutes(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListRoutesReply, error) {
	items, err := s.usecase.List(ctx)
	if err != nil {
		return nil, operationError(err, "查询路由失败")
	}
	reply := &adminv1.ListRoutesReply{Routes: make([]*adminv1.Route, 0, len(items))}
	for i := range items {
		reply.Routes = append(reply.Routes, routeReply(&items[i]))
	}
	return reply, nil
}

func (s *RouteService) GetRoute(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.Route, error) {
	item, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, operationError(err, "查询路由失败")
	}
	return routeReply(item), nil
}

func (s *RouteService) CreateRoute(ctx context.Context, request *adminv1.CreateRouteRequest) (*adminv1.MutationReply, error) {
	spec, err := routeSpec(request.GetName(), request.GetGatewayIds(), request.GetHostnames(), request.Enabled, request.GetRules())
	if err != nil {
		return nil, err
	}
	id, err := s.usecase.Create(ctx, spec)
	if err != nil {
		return nil, operationError(err, "创建路由失败")
	}
	return &adminv1.MutationReply{Success: true, Id: id}, nil
}

func (s *RouteService) UpdateRoute(ctx context.Context, request *adminv1.UpdateRouteRequest) (*adminv1.MutationReply, error) {
	spec, err := routeSpec(request.GetName(), request.GetGatewayIds(), request.GetHostnames(), request.Enabled, request.GetRules())
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec); err != nil {
		return nil, operationError(err, "更新路由失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *RouteService) SetRouteEnabled(ctx context.Context, request *adminv1.SetEnabledRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.SetEnabled(ctx, request.GetId(), request.GetEnabled()); err != nil {
		return nil, operationError(err, "更新路由状态失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *RouteService) DeleteRoute(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, operationError(err, "删除路由失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

// Package route 提供 Route 管理 API
package route

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	routebiz "github.com/lgc202/ingate/internal/adminapi/biz/route"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现路由管理 API
type Service struct {
	routes *routebiz.Service
}

// NewService 创建路由协议服务
func NewService(routes *routebiz.Service) *Service {
	return &Service{routes: routes}
}

func (s *Service) ListRoutes(ctx context.Context, request *adminv1.ListRoutesRequest) (*adminv1.ListRoutesResponse, error) {
	filter := routebiz.ListFilter{
		ResourceFilter: adminservice.ResourceFilter(request.GetQuery(), request.Enabled, request.GetState()),
	}
	switch request.GetType() {
	case adminv1.RouteType_ROUTE_TYPE_API:
		ai := false
		filter.AI = &ai
	case adminv1.RouteType_ROUTE_TYPE_AI:
		ai := true
		filter.AI = &ai
	}
	page, err := s.routes.List(ctx, adminservice.PageRequest(request.GetLimit(), request.GetCursor()), filter)
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListRoutesResponse{
		Routes:     make([]*adminv1.Route, 0, len(page.Items)),
		NextCursor: page.NextCursor,
	}
	for i := range page.Items {
		response.Routes = append(response.Routes, routeResponse(&page.Items[i]))
	}
	return response, nil
}

func (s *Service) GetRoute(ctx context.Context, request *adminv1.GetRouteRequest) (*adminv1.Route, error) {
	route, err := s.routes.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return routeResponse(route), nil
}

func (s *Service) CreateRoute(ctx context.Context, request *adminv1.CreateRouteRequest) (*adminv1.Route, error) {
	spec, err := createSpec(request)
	if err != nil {
		return nil, err
	}
	route, err := s.routes.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return routeResponse(route), nil
}

func (s *Service) UpdateRoute(ctx context.Context, request *adminv1.UpdateRouteRequest) (*adminv1.Route, error) {
	spec, err := updateSpec(request)
	if err != nil {
		return nil, err
	}
	route, err := s.routes.Update(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return routeResponse(route), nil
}

func (s *Service) DeleteRoute(ctx context.Context, request *adminv1.DeleteRouteRequest) (*emptypb.Empty, error) {
	if err := s.routes.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

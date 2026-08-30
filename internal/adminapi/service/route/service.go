// Package route 提供 Route 管理 API。
package route

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	routebiz "github.com/lgc202/ingate/internal/adminapi/biz/route"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现 Route 管理 API。
type Service struct {
	routes *routebiz.Usecase
}

// NewService 创建 Route 协议服务。
func NewService(routes *routebiz.Usecase) *Service {
	return &Service{routes: routes}
}

// ListRoutes 返回满足筛选条件的 Route 列表。
func (s *Service) ListRoutes(
	ctx context.Context,
	request *adminv1.ListRoutesRequest,
) (*adminv1.ListRoutesResponse, error) {
	filter := routebiz.ListFilter{
		ResourceFilter: adminservice.ResourceFilter(
			request.GetQuery(),
			request.Enabled,
			request.GetState(),
		),
	}
	switch request.GetType() {
	case adminv1.RouteType_ROUTE_TYPE_API:
		filter.Type = routebiz.TypeAPI
	case adminv1.RouteType_ROUTE_TYPE_AI:
		filter.Type = routebiz.TypeAI
	}
	page, err := s.routes.List(
		ctx,
		adminservice.PageRequest(request.GetLimit(), request.GetCursor()),
		filter,
	)
	if err != nil {
		return nil, err
	}
	routes := make([]*adminv1.Route, len(page.Items))
	for i := range page.Items {
		routes[i] = routeResponse(&page.Items[i])
	}
	return &adminv1.ListRoutesResponse{
		Routes:     routes,
		NextCursor: page.NextCursor,
	}, nil
}

// GetRoute 返回指定 Route。
func (s *Service) GetRoute(
	ctx context.Context,
	request *adminv1.GetRouteRequest,
) (*adminv1.Route, error) {
	route, err := s.routes.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return routeResponse(route), nil
}

// CreateRoute 创建 Route。
func (s *Service) CreateRoute(
	ctx context.Context,
	request *adminv1.CreateRouteRequest,
) (*adminv1.Route, error) {
	spec, err := parseRouteSpec(request.GetName(), request.GetConfig())
	if err != nil {
		return nil, err
	}
	route, err := s.routes.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return routeResponse(route), nil
}

// UpdateRoute 完整替换 Route 配置。
func (s *Service) UpdateRoute(
	ctx context.Context,
	request *adminv1.UpdateRouteRequest,
) (*adminv1.Route, error) {
	spec, err := parseRouteSpec(request.GetName(), request.GetConfig())
	if err != nil {
		return nil, err
	}
	route, err := s.routes.Replace(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return routeResponse(route), nil
}

// DeleteRoute 删除 Route。
func (s *Service) DeleteRoute(
	ctx context.Context,
	request *adminv1.DeleteRouteRequest,
) (*emptypb.Empty, error) {
	if err := s.routes.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

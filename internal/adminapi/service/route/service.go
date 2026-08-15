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
	business *routebiz.Service
}

// NewService 创建路由协议服务
func NewService(business *routebiz.Service) *Service {
	return &Service{business: business}
}

func (s *Service) ListRoutes(ctx context.Context, request *adminv1.ListRoutesRequest) (*adminv1.ListRoutesResponse, error) {
	result, err := s.business.List(ctx, adminservice.PageRequest(request.GetLimit(), request.GetCursor()))
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListRoutesResponse{
		Routes:     make([]*adminv1.Route, 0, len(result.Items)),
		NextCursor: result.NextCursor,
	}
	for i := range result.Items {
		response.Routes = append(response.Routes, routeFromResource(&result.Items[i]))
	}
	return response, nil
}

func (s *Service) GetRoute(ctx context.Context, request *adminv1.GetRouteRequest) (*adminv1.Route, error) {
	item, err := s.business.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return routeFromResource(item), nil
}

func (s *Service) CreateRoute(ctx context.Context, request *adminv1.CreateRouteRequest) (*adminv1.Route, error) {
	spec, err := buildRouteSpec(routeInput{
		name:                   request.GetName(),
		enabled:                request.GetEnabled(),
		gatewayIDs:             request.GetGatewayIds(),
		hostnames:              request.GetHostnames(),
		match:                  request.GetMatch(),
		upstreams:              request.GetUpstreams(),
		requestHeaderModifier:  request.GetRequestHeaderModifier(),
		responseHeaderModifier: request.GetResponseHeaderModifier(),
		timeout:                request.GetTimeout(),
		retry:                  request.GetRetry(),
	})
	if err != nil {
		return nil, err
	}
	item, err := s.business.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return routeFromResource(item), nil
}

func (s *Service) UpdateRoute(ctx context.Context, request *adminv1.UpdateRouteRequest) (*adminv1.Route, error) {
	spec, err := buildRouteSpec(routeInput{
		name:                   request.GetName(),
		enabled:                request.GetEnabled(),
		gatewayIDs:             request.GetGatewayIds(),
		hostnames:              request.GetHostnames(),
		match:                  request.GetMatch(),
		upstreams:              request.GetUpstreams(),
		requestHeaderModifier:  request.GetRequestHeaderModifier(),
		responseHeaderModifier: request.GetResponseHeaderModifier(),
		timeout:                request.GetTimeout(),
		retry:                  request.GetRetry(),
	})
	if err != nil {
		return nil, err
	}
	item, err := s.business.Update(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return routeFromResource(item), nil
}

func (s *Service) DeleteRoute(ctx context.Context, request *adminv1.DeleteRouteRequest) (*emptypb.Empty, error) {
	if err := s.business.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

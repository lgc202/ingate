package biz

import (
	"context"

	"github.com/lgc202/ingate/internal/adminapi/convert"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

type RouteService struct {
	store store.Store
}

func NewRouteService(store store.Store) *RouteService {
	return &RouteService{store: store}
}

func (s *RouteService) Create(ctx context.Context, req dto.CreateRouteRequest) (dto.RouteResponse, error) {
	created, err := s.store.CreateRoute(ctx, convert.RouteFromCreateRequest(req))
	if err != nil {
		return dto.RouteResponse{}, err
	}
	return convert.RouteToResponse(created), nil
}

func (s *RouteService) Update(ctx context.Context, name string, req dto.UpdateRouteRequest) (dto.RouteResponse, error) {
	current, err := s.store.GetRoute(ctx, name)
	if err != nil {
		return dto.RouteResponse{}, err
	}
	updated := convert.RouteFromUpdateRequest(name, req)
	updated.ObjectMeta = current.ObjectMeta
	updated.Status = current.Status
	result, err := s.store.UpdateRoute(ctx, updated)
	if err != nil {
		return dto.RouteResponse{}, err
	}
	return convert.RouteToResponse(result), nil
}

func (s *RouteService) Delete(ctx context.Context, name string) error {
	return s.store.DeleteRoute(ctx, name)
}

func (s *RouteService) Get(ctx context.Context, name string) (dto.RouteResponse, error) {
	route, err := s.store.GetRoute(ctx, name)
	if err != nil {
		return dto.RouteResponse{}, err
	}
	return convert.RouteToResponse(route), nil
}

func (s *RouteService) List(ctx context.Context) (dto.RouteListResponse, error) {
	list, err := s.store.ListRoutes(ctx)
	if err != nil {
		return dto.RouteListResponse{}, err
	}
	return convert.RouteListToResponse(list), nil
}

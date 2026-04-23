package biz

import (
	"context"

	"github.com/lgc202/ingate/internal/adminapi/convert"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

type GatewayService struct {
	store store.Store
}

func NewGatewayService(store store.Store) *GatewayService {
	return &GatewayService{store: store}
}

func (s *GatewayService) Create(ctx context.Context, req dto.CreateGatewayRequest) (dto.GatewayResponse, error) {
	created, err := s.store.CreateGateway(ctx, convert.GatewayFromCreateRequest(req))
	if err != nil {
		return dto.GatewayResponse{}, err
	}
	return convert.GatewayToResponse(created), nil
}

func (s *GatewayService) Update(ctx context.Context, name string, req dto.UpdateGatewayRequest) (dto.GatewayResponse, error) {
	current, err := s.store.GetGateway(ctx, name)
	if err != nil {
		return dto.GatewayResponse{}, err
	}
	updated := convert.GatewayFromUpdateRequest(name, req)
	updated.ObjectMeta = current.ObjectMeta
	updated.Status = current.Status
	result, err := s.store.UpdateGateway(ctx, updated)
	if err != nil {
		return dto.GatewayResponse{}, err
	}
	return convert.GatewayToResponse(result), nil
}

func (s *GatewayService) Delete(ctx context.Context, name string) error {
	return s.store.DeleteGateway(ctx, name)
}

func (s *GatewayService) Get(ctx context.Context, name string) (dto.GatewayResponse, error) {
	gateway, err := s.store.GetGateway(ctx, name)
	if err != nil {
		return dto.GatewayResponse{}, err
	}
	return convert.GatewayToResponse(gateway), nil
}

func (s *GatewayService) List(ctx context.Context) (dto.GatewayListResponse, error) {
	list, err := s.store.ListGateways(ctx)
	if err != nil {
		return dto.GatewayListResponse{}, err
	}
	return convert.GatewayListToResponse(list), nil
}

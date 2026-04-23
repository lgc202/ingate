package biz

import (
	"context"

	"github.com/lgc202/ingate/internal/adminapi/convert"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

type BackendService struct {
	store store.Store
}

func NewBackendService(store store.Store) *BackendService {
	return &BackendService{store: store}
}

func (s *BackendService) Create(ctx context.Context, req dto.CreateBackendRequest) (dto.BackendResponse, error) {
	created, err := s.store.CreateBackend(ctx, convert.BackendFromCreateRequest(req))
	if err != nil {
		return dto.BackendResponse{}, err
	}
	return convert.BackendToResponse(created), nil
}

func (s *BackendService) Update(ctx context.Context, name string, req dto.UpdateBackendRequest) (dto.BackendResponse, error) {
	current, err := s.store.GetBackend(ctx, name)
	if err != nil {
		return dto.BackendResponse{}, err
	}
	updated := convert.BackendFromUpdateRequest(name, req)
	updated.ObjectMeta = current.ObjectMeta
	updated.Status = current.Status
	result, err := s.store.UpdateBackend(ctx, updated)
	if err != nil {
		return dto.BackendResponse{}, err
	}
	return convert.BackendToResponse(result), nil
}

func (s *BackendService) Delete(ctx context.Context, name string) error {
	return s.store.DeleteBackend(ctx, name)
}

func (s *BackendService) Get(ctx context.Context, name string) (dto.BackendResponse, error) {
	backend, err := s.store.GetBackend(ctx, name)
	if err != nil {
		return dto.BackendResponse{}, err
	}
	return convert.BackendToResponse(backend), nil
}

func (s *BackendService) List(ctx context.Context) (dto.BackendListResponse, error) {
	list, err := s.store.ListBackends(ctx)
	if err != nil {
		return dto.BackendListResponse{}, err
	}
	return convert.BackendListToResponse(list), nil
}

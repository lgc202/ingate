package biz

import (
	"context"

	"github.com/lgc202/ingate/internal/adminapi/convert"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

type AuthPolicyService struct {
	store store.Store
}

func NewAuthPolicyService(store store.Store) *AuthPolicyService {
	return &AuthPolicyService{store: store}
}

func (s *AuthPolicyService) Create(ctx context.Context, req dto.CreateAuthPolicyRequest) (dto.AuthPolicyResponse, error) {
	created, err := s.store.CreateAuthPolicy(ctx, convert.AuthPolicyFromCreateRequest(req))
	if err != nil {
		return dto.AuthPolicyResponse{}, err
	}
	return convert.AuthPolicyToResponse(created), nil
}

func (s *AuthPolicyService) Update(ctx context.Context, name string, req dto.UpdateAuthPolicyRequest) (dto.AuthPolicyResponse, error) {
	current, err := s.store.GetAuthPolicy(ctx, name)
	if err != nil {
		return dto.AuthPolicyResponse{}, err
	}
	updated := convert.AuthPolicyFromUpdateRequest(name, req)
	updated.ObjectMeta = current.ObjectMeta
	updated.Status = current.Status
	result, err := s.store.UpdateAuthPolicy(ctx, updated)
	if err != nil {
		return dto.AuthPolicyResponse{}, err
	}
	return convert.AuthPolicyToResponse(result), nil
}

func (s *AuthPolicyService) Delete(ctx context.Context, name string) error {
	return s.store.DeleteAuthPolicy(ctx, name)
}

func (s *AuthPolicyService) Get(ctx context.Context, name string) (dto.AuthPolicyResponse, error) {
	policy, err := s.store.GetAuthPolicy(ctx, name)
	if err != nil {
		return dto.AuthPolicyResponse{}, err
	}
	return convert.AuthPolicyToResponse(policy), nil
}

func (s *AuthPolicyService) List(ctx context.Context) (dto.AuthPolicyListResponse, error) {
	list, err := s.store.ListAuthPolicies(ctx)
	if err != nil {
		return dto.AuthPolicyListResponse{}, err
	}
	return convert.AuthPolicyListToResponse(list), nil
}

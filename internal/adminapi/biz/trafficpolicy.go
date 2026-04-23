package biz

import (
	"context"

	"github.com/lgc202/ingate/internal/adminapi/convert"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

type TrafficPolicyService struct {
	store *store.APIServerStore
}

func NewTrafficPolicyService(store *store.APIServerStore) *TrafficPolicyService {
	return &TrafficPolicyService{store: store}
}

func (s *TrafficPolicyService) Create(ctx context.Context, req dto.CreateTrafficPolicyRequest) (dto.TrafficPolicyResponse, error) {
	created, err := s.store.CreateTrafficPolicy(ctx, convert.TrafficPolicyFromCreateRequest(req))
	if err != nil {
		return dto.TrafficPolicyResponse{}, err
	}
	return convert.TrafficPolicyToResponse(created), nil
}

func (s *TrafficPolicyService) Update(ctx context.Context, name string, req dto.UpdateTrafficPolicyRequest) (dto.TrafficPolicyResponse, error) {
	current, err := s.store.GetTrafficPolicy(ctx, name)
	if err != nil {
		return dto.TrafficPolicyResponse{}, err
	}
	updated := convert.TrafficPolicyFromUpdateRequest(name, req)
	updated.ObjectMeta = current.ObjectMeta
	updated.Status = current.Status
	result, err := s.store.UpdateTrafficPolicy(ctx, updated)
	if err != nil {
		return dto.TrafficPolicyResponse{}, err
	}
	return convert.TrafficPolicyToResponse(result), nil
}

func (s *TrafficPolicyService) Delete(ctx context.Context, name string) error {
	return s.store.DeleteTrafficPolicy(ctx, name)
}

func (s *TrafficPolicyService) Get(ctx context.Context, name string) (dto.TrafficPolicyResponse, error) {
	policy, err := s.store.GetTrafficPolicy(ctx, name)
	if err != nil {
		return dto.TrafficPolicyResponse{}, err
	}
	return convert.TrafficPolicyToResponse(policy), nil
}

func (s *TrafficPolicyService) List(ctx context.Context) (dto.TrafficPolicyListResponse, error) {
	list, err := s.store.ListTrafficPolicies(ctx)
	if err != nil {
		return dto.TrafficPolicyListResponse{}, err
	}
	return convert.TrafficPolicyListToResponse(list), nil
}

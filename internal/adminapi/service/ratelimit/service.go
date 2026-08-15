// Package ratelimit 提供请求限流策略管理 API
package ratelimit

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	ratelimitbiz "github.com/lgc202/ingate/internal/adminapi/biz/ratelimit"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现请求限流策略管理 API
type Service struct {
	business *ratelimitbiz.Service
}

// NewService 创建限流策略协议服务
func NewService(business *ratelimitbiz.Service) *Service {
	return &Service{business: business}
}

func (s *Service) ListRateLimitPolicies(
	ctx context.Context,
	request *adminv1.ListRateLimitPoliciesRequest,
) (*adminv1.ListRateLimitPoliciesResponse, error) {
	result, err := s.business.List(ctx, adminservice.PageRequest(request.GetLimit(), request.GetCursor()))
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListRateLimitPoliciesResponse{
		Policies:   make([]*adminv1.RateLimitPolicy, 0, len(result.Policies)),
		NextCursor: result.NextCursor,
	}
	for i := range result.Policies {
		response.Policies = append(
			response.Policies,
			rateLimitPolicyFromResource(&result.Policies[i], result.TargetNames),
		)
	}
	return response, nil
}

func (s *Service) GetRateLimitPolicy(
	ctx context.Context,
	request *adminv1.GetRateLimitPolicyRequest,
) (*adminv1.RateLimitPolicy, error) {
	result, err := s.business.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return rateLimitPolicyFromResource(result.Policy, result.TargetNames), nil
}

func (s *Service) CreateRateLimitPolicy(
	ctx context.Context,
	request *adminv1.CreateRateLimitPolicyRequest,
) (*adminv1.RateLimitPolicy, error) {
	spec, err := buildRateLimitPolicySpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetTargets(),
		request.GetSubject(),
		request.GetLimit(),
	)
	if err != nil {
		return nil, err
	}
	result, err := s.business.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return rateLimitPolicyFromResource(result.Policy, result.TargetNames), nil
}

func (s *Service) UpdateRateLimitPolicy(
	ctx context.Context,
	request *adminv1.UpdateRateLimitPolicyRequest,
) (*adminv1.RateLimitPolicy, error) {
	spec, err := buildRateLimitPolicySpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetTargets(),
		request.GetSubject(),
		request.GetLimit(),
	)
	if err != nil {
		return nil, err
	}
	result, err := s.business.Update(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return rateLimitPolicyFromResource(result.Policy, result.TargetNames), nil
}

func (s *Service) DeleteRateLimitPolicy(
	ctx context.Context,
	request *adminv1.DeleteRateLimitPolicyRequest,
) (*emptypb.Empty, error) {
	if err := s.business.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

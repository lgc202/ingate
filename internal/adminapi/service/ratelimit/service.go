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
	policies *ratelimitbiz.Service
}

// NewService 创建限流策略协议服务
func NewService(policies *ratelimitbiz.Service) *Service {
	return &Service{policies: policies}
}

func (s *Service) ListRateLimitPolicies(
	ctx context.Context,
	request *adminv1.ListRateLimitPoliciesRequest,
) (*adminv1.ListRateLimitPoliciesResponse, error) {
	page, err := s.policies.List(
		ctx,
		adminservice.PageRequest(request.GetLimit(), request.GetCursor()),
		adminservice.ResourceFilter(request.GetQuery(), request.Enabled, request.GetState()),
	)
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListRateLimitPoliciesResponse{
		Policies:   make([]*adminv1.RateLimitPolicy, 0, len(page.Policies)),
		NextCursor: page.NextCursor,
	}
	for i := range page.Policies {
		response.Policies = append(
			response.Policies,
			rateLimitPolicyResponse(&page.Policies[i], page.TargetNames),
		)
	}
	return response, nil
}

func (s *Service) GetRateLimitPolicy(
	ctx context.Context,
	request *adminv1.GetRateLimitPolicyRequest,
) (*adminv1.RateLimitPolicy, error) {
	view, err := s.policies.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return rateLimitPolicyResponse(view.Policy, view.TargetNames), nil
}

func (s *Service) CreateRateLimitPolicy(
	ctx context.Context,
	request *adminv1.CreateRateLimitPolicyRequest,
) (*adminv1.RateLimitPolicy, error) {
	spec, err := createSpec(request)
	if err != nil {
		return nil, err
	}
	view, err := s.policies.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return rateLimitPolicyResponse(view.Policy, view.TargetNames), nil
}

func (s *Service) UpdateRateLimitPolicy(
	ctx context.Context,
	request *adminv1.UpdateRateLimitPolicyRequest,
) (*adminv1.RateLimitPolicy, error) {
	spec, err := updateSpec(request)
	if err != nil {
		return nil, err
	}
	view, err := s.policies.Update(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return rateLimitPolicyResponse(view.Policy, view.TargetNames), nil
}

func (s *Service) DeleteRateLimitPolicy(
	ctx context.Context,
	request *adminv1.DeleteRateLimitPolicyRequest,
) (*emptypb.Empty, error) {
	if err := s.policies.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

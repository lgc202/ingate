// Package ratelimit 提供请求限流策略管理 API。
package ratelimit

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	ratelimitbiz "github.com/lgc202/ingate/internal/adminapi/biz/ratelimit"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
)

// Service 实现请求限流策略管理 API。
type Service struct {
	policies *ratelimitbiz.Usecase
}

// NewService 创建请求限流策略协议服务。
func NewService(policies *ratelimitbiz.Usecase) *Service {
	return &Service{policies: policies}
}

// ListRateLimitPolicies 返回满足筛选条件的请求限流策略。
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
	policies := make([]*adminv1.RateLimitPolicy, len(page.Items))
	for i := range page.Items {
		policies[i] = rateLimitPolicyResponse(&page.Items[i], page.TargetNames)
	}
	return &adminv1.ListRateLimitPoliciesResponse{
		Policies:   policies,
		NextCursor: page.NextCursor,
	}, nil
}

// GetRateLimitPolicy 返回指定请求限流策略。
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

// CreateRateLimitPolicy 创建请求限流策略。
func (s *Service) CreateRateLimitPolicy(
	ctx context.Context,
	request *adminv1.CreateRateLimitPolicyRequest,
) (*adminv1.RateLimitPolicy, error) {
	spec, err := parseRateLimitPolicySpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetTargets(),
		request.GetSubject(),
		request.GetLimit(),
	)
	if err != nil {
		return nil, err
	}
	view, err := s.policies.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return rateLimitPolicyResponse(view.Policy, view.TargetNames), nil
}

// UpdateRateLimitPolicy 完整替换请求限流策略配置。
func (s *Service) UpdateRateLimitPolicy(
	ctx context.Context,
	request *adminv1.UpdateRateLimitPolicyRequest,
) (*adminv1.RateLimitPolicy, error) {
	spec, err := parseRateLimitPolicySpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetTargets(),
		request.GetSubject(),
		request.GetLimit(),
	)
	if err != nil {
		return nil, err
	}
	view, err := s.policies.Replace(
		ctx,
		request.GetId(),
		request.GetVersion(),
		spec,
	)
	if err != nil {
		return nil, err
	}
	return rateLimitPolicyResponse(view.Policy, view.TargetNames), nil
}

// DeleteRateLimitPolicy 删除请求限流策略。
func (s *Service) DeleteRateLimitPolicy(
	ctx context.Context,
	request *adminv1.DeleteRateLimitPolicyRequest,
) (*emptypb.Empty, error) {
	if err := s.policies.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

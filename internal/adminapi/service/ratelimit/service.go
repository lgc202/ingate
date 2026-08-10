// Package ratelimit 实现请求限流策略管理 API
package ratelimit

import (
	"context"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	ratelimitbiz "github.com/lgc202/ingate/internal/adminapi/biz/ratelimit"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现请求限流策略管理 API
type Service struct {
	usecase *ratelimitbiz.Usecase
}

// NewService 创建限流策略协议服务
func NewService(usecase *ratelimitbiz.Usecase) *Service {
	return &Service{usecase: usecase}
}

func (s *Service) ListRateLimitPolicies(ctx context.Context, request *adminv1.ListRequest) (*adminv1.ListRateLimitPoliciesReply, error) {
	result, err := s.usecase.List(ctx, adminservice.PageRequest(request.GetPageSize(), request.GetPageToken()))
	if err != nil {
		return nil, err
	}
	reply := &adminv1.ListRateLimitPoliciesReply{Policies: make([]*adminv1.RateLimitPolicy, 0, len(result.Policies)), Page: adminservice.PageInfo(result.NextCursor)}
	for i := range result.Policies {
		reply.Policies = append(reply.Policies, newRateLimitPolicyReply(&result.Policies[i], result.TargetNames))
	}
	return reply, nil
}

func (s *Service) GetRateLimitPolicy(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.RateLimitPolicy, error) {
	result, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return newRateLimitPolicyReply(result.Policy, result.TargetNames), nil
}

func (s *Service) CreateRateLimitPolicy(ctx context.Context, request *adminv1.CreateRateLimitPolicyRequest) (*adminv1.MutationReply, error) {
	spec, err := buildRateLimitPolicySpec(
		request.GetName(), request.GetDescription(), request.GetEnabled(), request.GetTargets(),
		request.GetRules(), request.GetResponse(), request.GetFailurePolicy(),
	)
	if err != nil {
		return nil, err
	}
	id, err := s.usecase.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: id}, nil
}

func (s *Service) UpdateRateLimitPolicy(ctx context.Context, request *adminv1.UpdateRateLimitPolicyRequest) (*adminv1.MutationReply, error) {
	spec, err := buildRateLimitPolicySpec(
		request.GetName(), request.GetDescription(), request.GetEnabled(), request.GetTargets(),
		request.GetRules(), request.GetResponse(), request.GetFailurePolicy(),
	)
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) SetRateLimitPolicyEnabled(ctx context.Context, request *adminv1.SetEnabledRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.SetEnabled(ctx, request.GetId(), request.GetEnabled()); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) DeleteRateLimitPolicy(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

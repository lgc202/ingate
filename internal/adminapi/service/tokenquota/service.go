// Package tokenquota 实现 Token 配额策略管理 API
package tokenquota

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	tokenquotabiz "github.com/lgc202/ingate/internal/adminapi/biz/tokenquota"
)

// Service 实现模型 Token 配额策略管理 API
type Service struct {
	usecase *tokenquotabiz.Usecase
}

// NewService 创建 Token 配额策略协议服务
func NewService(usecase *tokenquotabiz.Usecase) *Service {
	return &Service{usecase: usecase}
}

func (s *Service) ListTokenQuotaPolicies(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListTokenQuotaPoliciesReply, error) {
	result, err := s.usecase.List(ctx)
	if err != nil {
		return nil, err
	}
	reply := &adminv1.ListTokenQuotaPoliciesReply{Policies: make([]*adminv1.TokenQuotaPolicy, 0, len(result.Policies))}
	for i := range result.Policies {
		reply.Policies = append(reply.Policies, newTokenQuotaPolicyReply(&result.Policies[i], result.TargetNames))
	}
	return reply, nil
}

func (s *Service) GetTokenQuotaPolicy(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.TokenQuotaPolicy, error) {
	result, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return newTokenQuotaPolicyReply(result.Policy, result.TargetNames), nil
}

func (s *Service) CreateTokenQuotaPolicy(ctx context.Context, request *adminv1.CreateTokenQuotaPolicyRequest) (*adminv1.MutationReply, error) {
	spec, err := buildTokenQuotaPolicySpec(
		request.GetName(), request.GetDescription(), request.GetEnabled(), request.GetTargets(),
		request.GetSubject(), request.GetQuota(), request.GetFailurePolicy(), request.GetResponse(),
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

func (s *Service) UpdateTokenQuotaPolicy(ctx context.Context, request *adminv1.UpdateTokenQuotaPolicyRequest) (*adminv1.MutationReply, error) {
	spec, err := buildTokenQuotaPolicySpec(
		request.GetName(), request.GetDescription(), request.GetEnabled(), request.GetTargets(),
		request.GetSubject(), request.GetQuota(), request.GetFailurePolicy(), request.GetResponse(),
	)
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) SetTokenQuotaPolicyEnabled(ctx context.Context, request *adminv1.SetEnabledRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.SetEnabled(ctx, request.GetId(), request.GetEnabled()); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) DeleteTokenQuotaPolicy(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

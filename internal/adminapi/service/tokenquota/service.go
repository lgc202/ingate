// Package tokenquota 提供调用方模型 Token 额度管理 API
package tokenquota

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	tokenquotabiz "github.com/lgc202/ingate/internal/adminapi/biz/tokenquota"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现 TokenQuotaPolicy 管理 API
type Service struct {
	policies *tokenquotabiz.Service
}

// NewService 创建 TokenQuotaPolicy 协议服务
func NewService(policies *tokenquotabiz.Service) *Service {
	return &Service{policies: policies}
}

func (s *Service) ListTokenQuotaPolicies(
	ctx context.Context,
	request *adminv1.ListTokenQuotaPoliciesRequest,
) (*adminv1.ListTokenQuotaPoliciesResponse, error) {
	page, err := s.policies.List(
		ctx,
		adminservice.PageRequest(request.GetLimit(), request.GetCursor()),
		adminservice.ResourceFilter(request.GetQuery(), request.Enabled, request.GetState()),
	)
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListTokenQuotaPoliciesResponse{
		Policies:   make([]*adminv1.TokenQuotaPolicy, 0, len(page.Policies)),
		NextCursor: page.NextCursor,
	}
	for i := range page.Policies {
		response.Policies = append(response.Policies, tokenQuotaPolicyResponse(&page.Policies[i], page.TargetNames))
	}
	return response, nil
}

func (s *Service) GetTokenQuotaPolicy(
	ctx context.Context,
	request *adminv1.GetTokenQuotaPolicyRequest,
) (*adminv1.TokenQuotaPolicy, error) {
	view, err := s.policies.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return tokenQuotaPolicyResponse(view.Policy, view.TargetNames), nil
}

func (s *Service) CreateTokenQuotaPolicy(
	ctx context.Context,
	request *adminv1.CreateTokenQuotaPolicyRequest,
) (*adminv1.TokenQuotaPolicy, error) {
	spec, err := createSpec(request)
	if err != nil {
		return nil, err
	}
	view, err := s.policies.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return tokenQuotaPolicyResponse(view.Policy, view.TargetNames), nil
}

func (s *Service) UpdateTokenQuotaPolicy(
	ctx context.Context,
	request *adminv1.UpdateTokenQuotaPolicyRequest,
) (*adminv1.TokenQuotaPolicy, error) {
	spec, err := updateSpec(request)
	if err != nil {
		return nil, err
	}
	view, err := s.policies.Update(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return tokenQuotaPolicyResponse(view.Policy, view.TargetNames), nil
}

func (s *Service) DeleteTokenQuotaPolicy(
	ctx context.Context,
	request *adminv1.DeleteTokenQuotaPolicyRequest,
) (*emptypb.Empty, error) {
	if err := s.policies.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// GetCallerTokenQuotaUsage 查询调用方当前实际执行的 Token 额度
func (s *Service) GetCallerTokenQuotaUsage(
	ctx context.Context,
	request *adminv1.GetCallerTokenQuotaUsageRequest,
) (*adminv1.GetCallerTokenQuotaUsageResponse, error) {
	usages, err := s.policies.CurrentUsage(ctx, request.GetCallerId())
	if err != nil {
		return nil, err
	}
	return tokenQuotaUsageResponse(usages), nil
}

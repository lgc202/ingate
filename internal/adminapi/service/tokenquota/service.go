// Package tokenquota 提供调用方模型 Token 额度管理 API。
package tokenquota

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	tokenquotabiz "github.com/lgc202/ingate/internal/adminapi/biz/tokenquota"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
)

// Service 实现 TokenQuotaPolicy 管理 API。
type Service struct {
	policies *tokenquotabiz.Usecase
}

// NewService 创建 TokenQuotaPolicy 协议服务。
func NewService(policies *tokenquotabiz.Usecase) *Service {
	return &Service{policies: policies}
}

// ListTokenQuotaPolicies 返回满足筛选条件的 Token 额度策略。
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
	policies := make([]*adminv1.TokenQuotaPolicy, len(page.Items))
	for i := range page.Items {
		policies[i] = tokenQuotaPolicyResponse(&page.Items[i], page.TargetNames)
	}
	return &adminv1.ListTokenQuotaPoliciesResponse{
		Policies:   policies,
		NextCursor: page.NextCursor,
	}, nil
}

// GetTokenQuotaPolicy 返回指定 Token 额度策略。
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

// CreateTokenQuotaPolicy 创建 Token 额度策略。
func (s *Service) CreateTokenQuotaPolicy(
	ctx context.Context,
	request *adminv1.CreateTokenQuotaPolicyRequest,
) (*adminv1.TokenQuotaPolicy, error) {
	spec, err := parseTokenQuotaPolicySpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetTargets(),
		request.GetTimeZone(),
		request.GetLimits(),
	)
	if err != nil {
		return nil, err
	}
	view, err := s.policies.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return tokenQuotaPolicyResponse(view.Policy, view.TargetNames), nil
}

// UpdateTokenQuotaPolicy 完整替换 Token 额度策略配置。
func (s *Service) UpdateTokenQuotaPolicy(
	ctx context.Context,
	request *adminv1.UpdateTokenQuotaPolicyRequest,
) (*adminv1.TokenQuotaPolicy, error) {
	spec, err := parseTokenQuotaPolicySpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetTargets(),
		request.GetTimeZone(),
		request.GetLimits(),
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
	return tokenQuotaPolicyResponse(view.Policy, view.TargetNames), nil
}

// DeleteTokenQuotaPolicy 删除 Token 额度策略。
func (s *Service) DeleteTokenQuotaPolicy(
	ctx context.Context,
	request *adminv1.DeleteTokenQuotaPolicyRequest,
) (*emptypb.Empty, error) {
	if err := s.policies.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// GetCallerTokenQuotaUsage 返回调用方当前实际执行的 Token 额度。
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

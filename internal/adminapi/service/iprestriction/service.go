// Package iprestriction 提供客户端 IP 访问限制策略管理 API。
package iprestriction

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	iprestrictionbiz "github.com/lgc202/ingate/internal/adminapi/biz/iprestriction"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
)

// Service 实现客户端 IP 访问限制策略管理 API。
type Service struct {
	policies *iprestrictionbiz.Usecase
}

// NewService 创建客户端 IP 访问限制策略协议服务。
func NewService(policies *iprestrictionbiz.Usecase) *Service {
	return &Service{policies: policies}
}

// ListIPRestrictionPolicies 返回满足筛选条件的客户端 IP 访问限制策略。
func (s *Service) ListIPRestrictionPolicies(
	ctx context.Context,
	request *adminv1.ListIPRestrictionPoliciesRequest,
) (*adminv1.ListIPRestrictionPoliciesResponse, error) {
	page, err := s.policies.List(
		ctx,
		adminservice.PageRequest(request.GetLimit(), request.GetCursor()),
		adminservice.ResourceFilter(request.GetQuery(), request.Enabled, request.GetState()),
	)
	if err != nil {
		return nil, err
	}
	policies := make([]*adminv1.IPRestrictionPolicy, len(page.Items))
	for i := range page.Items {
		policies[i] = ipRestrictionPolicyResponse(&page.Items[i], page.TargetNames)
	}
	return &adminv1.ListIPRestrictionPoliciesResponse{
		Policies:   policies,
		NextCursor: page.NextCursor,
	}, nil
}

// GetIPRestrictionPolicy 返回指定客户端 IP 访问限制策略。
func (s *Service) GetIPRestrictionPolicy(
	ctx context.Context,
	request *adminv1.GetIPRestrictionPolicyRequest,
) (*adminv1.IPRestrictionPolicy, error) {
	view, err := s.policies.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return ipRestrictionPolicyResponse(view.Policy, view.TargetNames), nil
}

// CreateIPRestrictionPolicy 创建并启用客户端 IP 访问限制策略。
func (s *Service) CreateIPRestrictionPolicy(
	ctx context.Context,
	request *adminv1.CreateIPRestrictionPolicyRequest,
) (*adminv1.IPRestrictionPolicy, error) {
	spec, err := parseIPRestrictionPolicySpec(
		request.GetName(),
		true,
		request.GetTargets(),
		request.GetAllow(),
		request.GetDeny(),
	)
	if err != nil {
		return nil, err
	}
	view, err := s.policies.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return ipRestrictionPolicyResponse(view.Policy, view.TargetNames), nil
}

// UpdateIPRestrictionPolicy 完整替换客户端 IP 访问限制策略配置。
func (s *Service) UpdateIPRestrictionPolicy(
	ctx context.Context,
	request *adminv1.UpdateIPRestrictionPolicyRequest,
) (*adminv1.IPRestrictionPolicy, error) {
	spec, err := parseIPRestrictionPolicySpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetTargets(),
		request.GetAllow(),
		request.GetDeny(),
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
	return ipRestrictionPolicyResponse(view.Policy, view.TargetNames), nil
}

// DeleteIPRestrictionPolicy 删除客户端 IP 访问限制策略。
func (s *Service) DeleteIPRestrictionPolicy(
	ctx context.Context,
	request *adminv1.DeleteIPRestrictionPolicyRequest,
) (*emptypb.Empty, error) {
	if err := s.policies.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

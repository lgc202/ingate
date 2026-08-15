// Package iprestriction 提供客户端 IP 访问限制策略管理 API
package iprestriction

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	iprestrictionbiz "github.com/lgc202/ingate/internal/adminapi/biz/iprestriction"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现客户端 IP 访问限制策略管理 API
type Service struct {
	business *iprestrictionbiz.Service
}

// NewService 创建客户端 IP 访问限制策略协议服务
func NewService(business *iprestrictionbiz.Service) *Service {
	return &Service{business: business}
}

func (s *Service) ListIPRestrictionPolicies(
	ctx context.Context,
	request *adminv1.ListIPRestrictionPoliciesRequest,
) (*adminv1.ListIPRestrictionPoliciesResponse, error) {
	result, err := s.business.List(ctx, adminservice.PageRequest(request.GetLimit(), request.GetCursor()))
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListIPRestrictionPoliciesResponse{
		Policies:   make([]*adminv1.IPRestrictionPolicy, 0, len(result.Policies)),
		NextCursor: result.NextCursor,
	}
	for i := range result.Policies {
		response.Policies = append(
			response.Policies,
			ipRestrictionPolicyFromResource(&result.Policies[i], result.TargetNames),
		)
	}
	return response, nil
}

func (s *Service) GetIPRestrictionPolicy(
	ctx context.Context,
	request *adminv1.GetIPRestrictionPolicyRequest,
) (*adminv1.IPRestrictionPolicy, error) {
	result, err := s.business.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return ipRestrictionPolicyFromResource(result.Policy, result.TargetNames), nil
}

func (s *Service) CreateIPRestrictionPolicy(
	ctx context.Context,
	request *adminv1.CreateIPRestrictionPolicyRequest,
) (*adminv1.IPRestrictionPolicy, error) {
	spec, err := buildIPRestrictionPolicySpec(
		request.GetName(),
		true,
		request.GetTargets(),
		request.GetAllow(),
		request.GetDeny(),
	)
	if err != nil {
		return nil, err
	}
	result, err := s.business.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return ipRestrictionPolicyFromResource(result.Policy, result.TargetNames), nil
}

func (s *Service) UpdateIPRestrictionPolicy(
	ctx context.Context,
	request *adminv1.UpdateIPRestrictionPolicyRequest,
) (*adminv1.IPRestrictionPolicy, error) {
	spec, err := buildIPRestrictionPolicySpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetTargets(),
		request.GetAllow(),
		request.GetDeny(),
	)
	if err != nil {
		return nil, err
	}
	result, err := s.business.Update(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return ipRestrictionPolicyFromResource(result.Policy, result.TargetNames), nil
}

func (s *Service) DeleteIPRestrictionPolicy(
	ctx context.Context,
	request *adminv1.DeleteIPRestrictionPolicyRequest,
) (*emptypb.Empty, error) {
	if err := s.business.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

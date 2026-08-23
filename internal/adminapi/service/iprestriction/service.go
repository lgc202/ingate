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
	policies *iprestrictionbiz.Service
}

// NewService 创建客户端 IP 访问限制策略协议服务
func NewService(policies *iprestrictionbiz.Service) *Service {
	return &Service{policies: policies}
}

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
	response := &adminv1.ListIPRestrictionPoliciesResponse{
		Policies:   make([]*adminv1.IPRestrictionPolicy, 0, len(page.Policies)),
		NextCursor: page.NextCursor,
	}
	for i := range page.Policies {
		response.Policies = append(
			response.Policies,
			ipRestrictionPolicyResponse(&page.Policies[i], page.TargetNames),
		)
	}
	return response, nil
}

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

func (s *Service) CreateIPRestrictionPolicy(
	ctx context.Context,
	request *adminv1.CreateIPRestrictionPolicyRequest,
) (*adminv1.IPRestrictionPolicy, error) {
	spec, err := createSpec(request)
	if err != nil {
		return nil, err
	}
	view, err := s.policies.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return ipRestrictionPolicyResponse(view.Policy, view.TargetNames), nil
}

func (s *Service) UpdateIPRestrictionPolicy(
	ctx context.Context,
	request *adminv1.UpdateIPRestrictionPolicyRequest,
) (*adminv1.IPRestrictionPolicy, error) {
	spec, err := updateSpec(request)
	if err != nil {
		return nil, err
	}
	view, err := s.policies.Update(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return ipRestrictionPolicyResponse(view.Policy, view.TargetNames), nil
}

func (s *Service) DeleteIPRestrictionPolicy(
	ctx context.Context,
	request *adminv1.DeleteIPRestrictionPolicyRequest,
) (*emptypb.Empty, error) {
	if err := s.policies.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

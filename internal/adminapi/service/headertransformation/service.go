// Package headertransformation 提供请求响应 Header 转换策略管理 API
package headertransformation

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	headertransformationbiz "github.com/lgc202/ingate/internal/adminapi/biz/headertransformation"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现请求响应 Header 转换策略管理 API
type Service struct {
	policies *headertransformationbiz.Service
}

// NewService 创建请求响应 Header 转换策略协议服务
func NewService(policies *headertransformationbiz.Service) *Service {
	return &Service{policies: policies}
}

func (s *Service) ListHeaderTransformationPolicies(
	ctx context.Context,
	request *adminv1.ListHeaderTransformationPoliciesRequest,
) (*adminv1.ListHeaderTransformationPoliciesResponse, error) {
	page, err := s.policies.List(ctx, adminservice.PageRequest(request.GetLimit(), request.GetCursor()))
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListHeaderTransformationPoliciesResponse{
		Policies:   make([]*adminv1.HeaderTransformationPolicy, 0, len(page.Policies)),
		NextCursor: page.NextCursor,
	}
	for i := range page.Policies {
		response.Policies = append(response.Policies, policyResponse(&page.Policies[i], page.TargetNames))
	}
	return response, nil
}

func (s *Service) GetHeaderTransformationPolicy(
	ctx context.Context,
	request *adminv1.GetHeaderTransformationPolicyRequest,
) (*adminv1.HeaderTransformationPolicy, error) {
	view, err := s.policies.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return policyResponse(view.Policy, view.TargetNames), nil
}

func (s *Service) CreateHeaderTransformationPolicy(
	ctx context.Context,
	request *adminv1.CreateHeaderTransformationPolicyRequest,
) (*adminv1.HeaderTransformationPolicy, error) {
	spec, err := createSpec(request)
	if err != nil {
		return nil, err
	}
	view, err := s.policies.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return policyResponse(view.Policy, view.TargetNames), nil
}

func (s *Service) UpdateHeaderTransformationPolicy(
	ctx context.Context,
	request *adminv1.UpdateHeaderTransformationPolicyRequest,
) (*adminv1.HeaderTransformationPolicy, error) {
	spec, err := updateSpec(request)
	if err != nil {
		return nil, err
	}
	view, err := s.policies.Update(ctx, request.GetId(), request.GetVersion(), spec)
	if err != nil {
		return nil, err
	}
	return policyResponse(view.Policy, view.TargetNames), nil
}

func (s *Service) DeleteHeaderTransformationPolicy(
	ctx context.Context,
	request *adminv1.DeleteHeaderTransformationPolicyRequest,
) (*emptypb.Empty, error) {
	if err := s.policies.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

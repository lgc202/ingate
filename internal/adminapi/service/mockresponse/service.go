// Package mockresponse 提供模拟响应策略管理 API
package mockresponse

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	mockresponsebiz "github.com/lgc202/ingate/internal/adminapi/biz/mockresponse"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现模拟响应策略管理 API
type Service struct {
	policies *mockresponsebiz.Service
}

// NewService 创建模拟响应策略协议服务
func NewService(policies *mockresponsebiz.Service) *Service {
	return &Service{policies: policies}
}

func (s *Service) ListMockResponsePolicies(
	ctx context.Context,
	request *adminv1.ListMockResponsePoliciesRequest,
) (*adminv1.ListMockResponsePoliciesResponse, error) {
	page, err := s.policies.List(ctx, adminservice.PageRequest(request.GetLimit(), request.GetCursor()))
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListMockResponsePoliciesResponse{
		Policies:   make([]*adminv1.MockResponsePolicy, 0, len(page.Policies)),
		NextCursor: page.NextCursor,
	}
	for i := range page.Policies {
		response.Policies = append(response.Policies, policyResponse(&page.Policies[i], page.TargetNames))
	}
	return response, nil
}

func (s *Service) GetMockResponsePolicy(
	ctx context.Context,
	request *adminv1.GetMockResponsePolicyRequest,
) (*adminv1.MockResponsePolicy, error) {
	view, err := s.policies.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return policyResponse(view.Policy, view.TargetNames), nil
}

func (s *Service) CreateMockResponsePolicy(
	ctx context.Context,
	request *adminv1.CreateMockResponsePolicyRequest,
) (*adminv1.MockResponsePolicy, error) {
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

func (s *Service) UpdateMockResponsePolicy(
	ctx context.Context,
	request *adminv1.UpdateMockResponsePolicyRequest,
) (*adminv1.MockResponsePolicy, error) {
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

func (s *Service) DeleteMockResponsePolicy(
	ctx context.Context,
	request *adminv1.DeleteMockResponsePolicyRequest,
) (*emptypb.Empty, error) {
	if err := s.policies.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

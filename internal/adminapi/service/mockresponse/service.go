// Package mockresponse 提供模拟响应策略管理 API。
package mockresponse

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	mockresponsebiz "github.com/lgc202/ingate/internal/adminapi/biz/mockresponse"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
)

// Service 实现模拟响应策略管理 API。
type Service struct {
	policies *mockresponsebiz.Usecase
}

// NewService 创建模拟响应策略协议服务。
func NewService(policies *mockresponsebiz.Usecase) *Service {
	return &Service{policies: policies}
}

// ListMockResponsePolicies 返回满足筛选条件的模拟响应策略。
func (s *Service) ListMockResponsePolicies(
	ctx context.Context,
	request *adminv1.ListMockResponsePoliciesRequest,
) (*adminv1.ListMockResponsePoliciesResponse, error) {
	page, err := s.policies.List(
		ctx,
		adminservice.PageRequest(request.GetLimit(), request.GetCursor()),
		adminservice.ResourceFilter(request.GetQuery(), request.Enabled, request.GetState()),
	)
	if err != nil {
		return nil, err
	}
	policies := make([]*adminv1.MockResponsePolicy, len(page.Items))
	for i := range page.Items {
		policies[i] = mockResponsePolicyResponse(&page.Items[i], page.TargetNames)
	}
	return &adminv1.ListMockResponsePoliciesResponse{
		Policies:   policies,
		NextCursor: page.NextCursor,
	}, nil
}

// GetMockResponsePolicy 返回指定模拟响应策略。
func (s *Service) GetMockResponsePolicy(
	ctx context.Context,
	request *adminv1.GetMockResponsePolicyRequest,
) (*adminv1.MockResponsePolicy, error) {
	view, err := s.policies.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return mockResponsePolicyResponse(view.Policy, view.TargetNames), nil
}

// CreateMockResponsePolicy 创建并启用模拟响应策略。
func (s *Service) CreateMockResponsePolicy(
	ctx context.Context,
	request *adminv1.CreateMockResponsePolicyRequest,
) (*adminv1.MockResponsePolicy, error) {
	spec, err := parseMockResponsePolicySpec(
		request.GetName(),
		true,
		request.GetTargets(),
		request.GetStatusCode(),
		request.GetContentType(),
		request.GetHeaders(),
		request.GetBody(),
	)
	if err != nil {
		return nil, err
	}
	view, err := s.policies.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return mockResponsePolicyResponse(view.Policy, view.TargetNames), nil
}

// UpdateMockResponsePolicy 完整替换模拟响应策略配置。
func (s *Service) UpdateMockResponsePolicy(
	ctx context.Context,
	request *adminv1.UpdateMockResponsePolicyRequest,
) (*adminv1.MockResponsePolicy, error) {
	spec, err := parseMockResponsePolicySpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetTargets(),
		request.GetStatusCode(),
		request.GetContentType(),
		request.GetHeaders(),
		request.GetBody(),
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
	return mockResponsePolicyResponse(view.Policy, view.TargetNames), nil
}

// DeleteMockResponsePolicy 删除模拟响应策略。
func (s *Service) DeleteMockResponsePolicy(
	ctx context.Context,
	request *adminv1.DeleteMockResponsePolicyRequest,
) (*emptypb.Empty, error) {
	if err := s.policies.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

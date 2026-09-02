// Package headertransformation 提供请求响应 Header 转换策略管理 API。
package headertransformation

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	headertransformationbiz "github.com/lgc202/ingate/internal/adminapi/biz/headertransformation"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
)

// Service 实现请求响应 Header 转换策略管理 API。
type Service struct {
	policies *headertransformationbiz.Usecase
}

// NewService 创建请求响应 Header 转换策略协议服务。
func NewService(policies *headertransformationbiz.Usecase) *Service {
	return &Service{policies: policies}
}

// ListHeaderTransformationPolicies 返回满足筛选条件的请求响应 Header 转换策略。
func (s *Service) ListHeaderTransformationPolicies(
	ctx context.Context,
	request *adminv1.ListHeaderTransformationPoliciesRequest,
) (*adminv1.ListHeaderTransformationPoliciesResponse, error) {
	page, err := s.policies.List(
		ctx,
		adminservice.PageRequest(request.GetLimit(), request.GetCursor()),
		adminservice.ResourceFilter(request.GetQuery(), request.Enabled, request.GetState()),
	)
	if err != nil {
		return nil, err
	}
	policies := make([]*adminv1.HeaderTransformationPolicy, len(page.Items))
	for i := range page.Items {
		policies[i] = headerTransformationPolicyResponse(
			&page.Items[i],
			page.TargetNames,
		)
	}
	return &adminv1.ListHeaderTransformationPoliciesResponse{
		Policies:   policies,
		NextCursor: page.NextCursor,
	}, nil
}

// GetHeaderTransformationPolicy 返回指定请求响应 Header 转换策略。
func (s *Service) GetHeaderTransformationPolicy(
	ctx context.Context,
	request *adminv1.GetHeaderTransformationPolicyRequest,
) (*adminv1.HeaderTransformationPolicy, error) {
	view, err := s.policies.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return headerTransformationPolicyResponse(view.Policy, view.TargetNames), nil
}

// CreateHeaderTransformationPolicy 创建并启用请求响应 Header 转换策略。
func (s *Service) CreateHeaderTransformationPolicy(
	ctx context.Context,
	request *adminv1.CreateHeaderTransformationPolicyRequest,
) (*adminv1.HeaderTransformationPolicy, error) {
	spec, err := parseHeaderTransformationPolicySpec(
		request.GetName(),
		true,
		request.GetTargets(),
		request.GetRequestRules(),
		request.GetResponseRules(),
	)
	if err != nil {
		return nil, err
	}
	view, err := s.policies.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return headerTransformationPolicyResponse(view.Policy, view.TargetNames), nil
}

// UpdateHeaderTransformationPolicy 完整替换请求响应 Header 转换策略配置。
func (s *Service) UpdateHeaderTransformationPolicy(
	ctx context.Context,
	request *adminv1.UpdateHeaderTransformationPolicyRequest,
) (*adminv1.HeaderTransformationPolicy, error) {
	spec, err := parseHeaderTransformationPolicySpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetTargets(),
		request.GetRequestRules(),
		request.GetResponseRules(),
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
	return headerTransformationPolicyResponse(view.Policy, view.TargetNames), nil
}

// DeleteHeaderTransformationPolicy 删除请求响应 Header 转换策略。
func (s *Service) DeleteHeaderTransformationPolicy(
	ctx context.Context,
	request *adminv1.DeleteHeaderTransformationPolicyRequest,
) (*emptypb.Empty, error) {
	if err := s.policies.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

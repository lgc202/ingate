// Package accesscontrol 实现访问控制策略管理 API
package accesscontrol

import (
	"context"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	accesscontrolbiz "github.com/lgc202/ingate/internal/adminapi/biz/accesscontrol"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现访问控制策略管理 API
type Service struct {
	usecase *accesscontrolbiz.Usecase
}

// NewService 创建访问控制策略协议服务
func NewService(usecase *accesscontrolbiz.Usecase) *Service {
	return &Service{usecase: usecase}
}

func (s *Service) ListAccessControlPolicies(ctx context.Context, request *adminv1.ListRequest) (*adminv1.ListAccessControlPoliciesReply, error) {
	result, err := s.usecase.List(ctx, adminservice.PageRequest(request.GetPageSize(), request.GetPageToken()))
	if err != nil {
		return nil, err
	}
	reply := &adminv1.ListAccessControlPoliciesReply{Policies: make([]*adminv1.AccessControlPolicy, 0, len(result.Policies)), Page: adminservice.PageInfo(result.NextCursor)}
	for i := range result.Policies {
		reply.Policies = append(reply.Policies, newAccessControlPolicyReply(&result.Policies[i], result.TargetNames))
	}
	return reply, nil
}

func (s *Service) GetAccessControlPolicy(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.AccessControlPolicy, error) {
	result, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return newAccessControlPolicyReply(result.Policy, result.TargetNames), nil
}

func (s *Service) CreateAccessControlPolicy(ctx context.Context, request *adminv1.CreateAccessControlPolicyRequest) (*adminv1.MutationReply, error) {
	spec, err := buildAccessControlPolicySpec(
		request.GetName(), request.GetDescription(), request.GetEnabled(), request.GetTargets(),
		request.GetDefaultAction(), request.GetRules(), request.GetResponse(),
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

func (s *Service) UpdateAccessControlPolicy(ctx context.Context, request *adminv1.UpdateAccessControlPolicyRequest) (*adminv1.MutationReply, error) {
	spec, err := buildAccessControlPolicySpec(
		request.GetName(), request.GetDescription(), request.GetEnabled(), request.GetTargets(),
		request.GetDefaultAction(), request.GetRules(), request.GetResponse(),
	)
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) SetAccessControlPolicyEnabled(ctx context.Context, request *adminv1.SetEnabledRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.SetEnabled(ctx, request.GetId(), request.GetEnabled()); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) DeleteAccessControlPolicy(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

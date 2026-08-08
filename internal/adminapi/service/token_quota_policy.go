package service

import (
	"context"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// TokenQuotaPolicyService 实现模型 Token 配额策略管理 API
type TokenQuotaPolicyService struct {
	usecase *biz.TokenQuotaPolicyUsecase
}

// NewTokenQuotaPolicyService 创建 Token 配额策略协议服务
func NewTokenQuotaPolicyService(usecase *biz.TokenQuotaPolicyUsecase) *TokenQuotaPolicyService {
	return &TokenQuotaPolicyService{usecase: usecase}
}

func (s *TokenQuotaPolicyService) ListTokenQuotaPolicies(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListTokenQuotaPoliciesReply, error) {
	result, err := s.usecase.List(ctx)
	if err != nil {
		return nil, operationError(err, "查询 Token 配额策略失败")
	}
	reply := &adminv1.ListTokenQuotaPoliciesReply{Policies: make([]*adminv1.TokenQuotaPolicy, 0, len(result.Policies))}
	for i := range result.Policies {
		reply.Policies = append(reply.Policies, tokenQuotaPolicyReply(&result.Policies[i], result.TargetNames))
	}
	return reply, nil
}

func (s *TokenQuotaPolicyService) GetTokenQuotaPolicy(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.TokenQuotaPolicy, error) {
	result, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, operationError(err, "查询 Token 配额策略失败")
	}
	return tokenQuotaPolicyReply(result.Policy, result.TargetNames), nil
}

func (s *TokenQuotaPolicyService) CreateTokenQuotaPolicy(ctx context.Context, request *adminv1.CreateTokenQuotaPolicyRequest) (*adminv1.MutationReply, error) {
	spec, err := tokenQuotaPolicySpec(
		request.GetName(), request.GetDescription(), request.GetEnabled(), request.GetTargets(),
		request.GetSubject(), request.GetQuota(), request.GetFailurePolicy(), request.GetResponse(),
	)
	if err != nil {
		return nil, err
	}
	id, err := s.usecase.Create(ctx, spec)
	if err != nil {
		return nil, operationError(err, "创建 Token 配额策略失败")
	}
	return &adminv1.MutationReply{Success: true, Id: id}, nil
}

func (s *TokenQuotaPolicyService) UpdateTokenQuotaPolicy(ctx context.Context, request *adminv1.UpdateTokenQuotaPolicyRequest) (*adminv1.MutationReply, error) {
	spec, err := tokenQuotaPolicySpec(
		request.GetName(), request.GetDescription(), request.GetEnabled(), request.GetTargets(),
		request.GetSubject(), request.GetQuota(), request.GetFailurePolicy(), request.GetResponse(),
	)
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec); err != nil {
		return nil, operationError(err, "更新 Token 配额策略失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *TokenQuotaPolicyService) SetTokenQuotaPolicyEnabled(ctx context.Context, request *adminv1.SetEnabledRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.SetEnabled(ctx, request.GetId(), request.GetEnabled()); err != nil {
		return nil, operationError(err, "更新 Token 配额策略状态失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *TokenQuotaPolicyService) DeleteTokenQuotaPolicy(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, operationError(err, "删除 Token 配额策略失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func tokenQuotaPolicySpec(
	name, description string,
	enabled bool,
	targets []*adminv1.PolicyTargetRef,
	subject *adminv1.TokenQuotaSubject,
	quota *adminv1.TokenQuota,
	failurePolicy string,
	response *adminv1.TokenQuotaResponse,
) (resource.TokenQuotaPolicySpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.TokenQuotaPolicySpec{}, badRequest("名称不能为空")
	}
	refs, err := policyTargetRefs(targets)
	if err != nil {
		return resource.TokenQuotaPolicySpec{}, err
	}
	if subject == nil || quota == nil {
		return resource.TokenQuotaPolicySpec{}, badRequest("额度划分方式和 Token 额度不能为空")
	}
	subjectValue := resource.TokenQuotaSubject{
		Type: resource.TokenQuotaSubjectType(subject.GetType()), HeaderName: strings.TrimSpace(subject.GetHeaderName()),
	}
	switch subjectValue.Type {
	case resource.TokenQuotaSubjectTypeShared, resource.TokenQuotaSubjectTypeIP:
		subjectValue.HeaderName = ""
	case resource.TokenQuotaSubjectTypeHeader:
		if subjectValue.HeaderName == "" || len(k8svalidation.IsHTTPHeaderName(subjectValue.HeaderName)) > 0 {
			return resource.TokenQuotaPolicySpec{}, badRequest("请求头名称不正确")
		}
	default:
		return resource.TokenQuotaPolicySpec{}, badRequest("额度划分方式不正确")
	}
	if quota.GetTokens() <= 0 || quota.GetTokens() > resource.TokenQuotaMaxTokens {
		return resource.TokenQuotaPolicySpec{}, badRequest("Token 额度超出支持范围")
	}
	if quota.GetWindowSeconds() <= 0 || quota.GetWindowSeconds() > resource.TokenQuotaMaxWindowSeconds {
		return resource.TokenQuotaPolicySpec{}, badRequest("统计周期超出支持范围")
	}
	policy := resource.TokenQuotaFailurePolicy(failurePolicy)
	if policy != resource.TokenQuotaFailurePolicyFailOpen && policy != resource.TokenQuotaFailurePolicyFailClose {
		return resource.TokenQuotaPolicySpec{}, badRequest("失败策略不正确")
	}
	spec := resource.TokenQuotaPolicySpec{
		DisplayName: name, Description: description, Enabled: enabled, TargetRefs: refs,
		Subject:       subjectValue,
		Quota:         resource.TokenQuota{Tokens: quota.GetTokens(), WindowSeconds: quota.GetWindowSeconds()},
		FailurePolicy: policy,
	}
	if response != nil {
		spec.Response.Message = response.GetMessage()
	}
	return spec, nil
}

func tokenQuotaPolicyReply(policy *resource.TokenQuotaPolicy, names biz.PolicyTargetNames) *adminv1.TokenQuotaPolicy {
	status := biz.PolicyStatus(policy.Generation, policy.Spec.Enabled, len(policy.Spec.TargetRefs), policy.Status.Conditions)
	disabled := status.State == biz.ResourceStateDisabled
	reply := &adminv1.TokenQuotaPolicy{
		Id: policy.Name, Version: strconv.FormatInt(policy.Generation, 10), Status: resourceStatus(status),
		Name: policy.Spec.DisplayName, Description: policy.Spec.Description, Enabled: policy.Spec.Enabled,
		Targets: policyTargets(policy.Generation, disabled, policy.Spec.TargetRefs, policy.Status.Targets, names),
		Subject: &adminv1.TokenQuotaSubject{
			Type: string(policy.Spec.Subject.Type), HeaderName: policy.Spec.Subject.HeaderName,
		},
		Quota: &adminv1.TokenQuota{
			Tokens: policy.Spec.Quota.Tokens, WindowSeconds: policy.Spec.Quota.WindowSeconds,
		},
		FailurePolicy: string(policy.Spec.FailurePolicy),
		Response:      &adminv1.TokenQuotaResponse{Message: policy.Spec.Response.Message},
		CreatedAt:     timestamp(policy.CreationTimestamp.Time),
	}
	if reply.Response.Message == "" {
		reply.Response.Message = "Token quota exceeded"
	}
	return reply
}

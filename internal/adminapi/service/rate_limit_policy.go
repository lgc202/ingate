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

// RateLimitPolicyService 实现请求限流策略管理 API
type RateLimitPolicyService struct {
	usecase *biz.RateLimitPolicyUsecase
}

// NewRateLimitPolicyService 创建限流策略协议服务
func NewRateLimitPolicyService(usecase *biz.RateLimitPolicyUsecase) *RateLimitPolicyService {
	return &RateLimitPolicyService{usecase: usecase}
}

func (s *RateLimitPolicyService) ListRateLimitPolicies(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListRateLimitPoliciesReply, error) {
	result, err := s.usecase.List(ctx)
	if err != nil {
		return nil, operationError(err, "查询限流策略失败")
	}
	reply := &adminv1.ListRateLimitPoliciesReply{Policies: make([]*adminv1.RateLimitPolicy, 0, len(result.Policies))}
	for i := range result.Policies {
		reply.Policies = append(reply.Policies, rateLimitPolicyReply(&result.Policies[i], result.TargetNames))
	}
	return reply, nil
}

func (s *RateLimitPolicyService) GetRateLimitPolicy(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.RateLimitPolicy, error) {
	if err := validateID(request.GetId()); err != nil {
		return nil, err
	}
	result, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, operationError(err, "查询限流策略失败")
	}
	return rateLimitPolicyReply(result.Policy, result.TargetNames), nil
}

func (s *RateLimitPolicyService) CreateRateLimitPolicy(ctx context.Context, request *adminv1.CreateRateLimitPolicyRequest) (*adminv1.MutationReply, error) {
	spec, err := rateLimitPolicySpec(
		request.GetName(), request.GetDescription(), request.GetEnabled(), request.GetTargets(),
		request.GetRules(), request.GetResponse(), request.GetFailurePolicy(),
	)
	if err != nil {
		return nil, err
	}
	id, err := s.usecase.Create(ctx, spec)
	if err != nil {
		return nil, operationError(err, "创建限流策略失败")
	}
	return &adminv1.MutationReply{Success: true, Id: id}, nil
}

func (s *RateLimitPolicyService) UpdateRateLimitPolicy(ctx context.Context, request *adminv1.UpdateRateLimitPolicyRequest) (*adminv1.MutationReply, error) {
	if err := validateID(request.GetId()); err != nil {
		return nil, err
	}
	if request.GetVersion() == "" {
		return nil, badRequest("版本不能为空")
	}
	spec, err := rateLimitPolicySpec(
		request.GetName(), request.GetDescription(), request.GetEnabled(), request.GetTargets(),
		request.GetRules(), request.GetResponse(), request.GetFailurePolicy(),
	)
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec); err != nil {
		return nil, operationError(err, "更新限流策略失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *RateLimitPolicyService) SetRateLimitPolicyEnabled(ctx context.Context, request *adminv1.SetEnabledRequest) (*adminv1.MutationReply, error) {
	if err := validateID(request.GetId()); err != nil {
		return nil, err
	}
	if request.Enabled == nil {
		return nil, badRequest("启用状态不能为空")
	}
	if err := s.usecase.SetEnabled(ctx, request.GetId(), request.GetEnabled()); err != nil {
		return nil, operationError(err, "更新限流策略状态失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *RateLimitPolicyService) DeleteRateLimitPolicy(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := validateID(request.GetId()); err != nil {
		return nil, err
	}
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, operationError(err, "删除限流策略失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func rateLimitPolicySpec(
	name, description string,
	enabled bool,
	targets []*adminv1.PolicyTargetRef,
	rules []*adminv1.RateLimitRule,
	response *adminv1.RateLimitResponse,
	failurePolicy string,
) (resource.RateLimitPolicySpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.RateLimitPolicySpec{}, badRequest("名称不能为空")
	}
	refs, err := policyTargetRefs(targets)
	if err != nil {
		return resource.RateLimitPolicySpec{}, err
	}
	if len(rules) == 0 {
		return resource.RateLimitPolicySpec{}, badRequest("至少需要一条限流规则")
	}
	spec := resource.RateLimitPolicySpec{
		DisplayName: name, Description: description, Enabled: enabled, TargetRefs: refs,
		FailurePolicy: resource.RateLimitFailurePolicy(failurePolicy),
	}
	if spec.FailurePolicy == "" {
		spec.FailurePolicy = resource.RateLimitFailurePolicyFailOpen
	}
	if spec.FailurePolicy != resource.RateLimitFailurePolicyFailOpen && spec.FailurePolicy != resource.RateLimitFailurePolicyFailClose {
		return resource.RateLimitPolicySpec{}, badRequest("失败策略不正确")
	}
	seen := make(map[string]struct{}, len(rules))
	for _, input := range rules {
		if input == nil || strings.TrimSpace(input.GetName()) == "" || input.GetKey() == nil || input.GetLimit() == nil {
			return resource.RateLimitPolicySpec{}, badRequest("限流规则配置不完整")
		}
		ruleName := strings.TrimSpace(input.GetName())
		if _, exists := seen[ruleName]; exists {
			return resource.RateLimitPolicySpec{}, badRequest("限流规则名称不能重复")
		}
		seen[ruleName] = struct{}{}
		limit := input.GetLimit()
		if limit.GetRequests() <= 0 || limit.GetWindowSeconds() <= 0 || limit.GetBurst() < 0 {
			return resource.RateLimitPolicySpec{}, badRequest("限流额度必须大于 0")
		}
		rule := resource.RateLimitRule{Name: ruleName, Limit: resource.RateLimitQuota{
			Requests: int(limit.GetRequests()), WindowSeconds: int(limit.GetWindowSeconds()), Burst: int(limit.GetBurst()),
		}}
		if len(input.GetKey().GetParts()) == 0 {
			return resource.RateLimitPolicySpec{}, badRequest("限流规则必须配置计数维度")
		}
		for _, inputPart := range input.GetKey().GetParts() {
			if inputPart == nil {
				return resource.RateLimitPolicySpec{}, badRequest("限流维度不能为空")
			}
			part := resource.RateLimitKeyPart{Type: resource.RateLimitKeyType(inputPart.GetType()), Name: strings.TrimSpace(inputPart.GetName())}
			switch part.Type {
			case resource.RateLimitKeyTypeIP, resource.RateLimitKeyTypeRoute, resource.RateLimitKeyTypeGateway, resource.RateLimitKeyTypeRouteRule:
				part.Name = ""
			case resource.RateLimitKeyTypeHeader:
				if part.Name == "" {
					return resource.RateLimitPolicySpec{}, badRequest("限流维度名称不能为空")
				}
				if len(k8svalidation.IsHTTPHeaderName(part.Name)) > 0 {
					return resource.RateLimitPolicySpec{}, badRequest("请求头名称不正确")
				}
			case resource.RateLimitKeyTypeQuery, resource.RateLimitKeyTypeCookie:
				if part.Name == "" {
					return resource.RateLimitPolicySpec{}, badRequest("限流维度名称不能为空")
				}
			default:
				return resource.RateLimitPolicySpec{}, badRequest("限流维度不正确")
			}
			rule.Key.Parts = append(rule.Key.Parts, part)
		}
		spec.Rules = append(spec.Rules, rule)
	}
	if response != nil {
		if response.GetStatusCode() != 0 && (response.GetStatusCode() < 400 || response.GetStatusCode() > 599) {
			return resource.RateLimitPolicySpec{}, badRequest("超限响应状态码必须在 400 到 599 之间")
		}
		spec.Response = resource.RateLimitResponse{
			StatusCode: int(response.GetStatusCode()), Message: response.GetMessage(),
			QuotaHeaderEnabled: response.GetQuotaHeaderEnabled(),
		}
	}
	return spec, nil
}

func rateLimitPolicyReply(policy *resource.RateLimitPolicy, names biz.PolicyTargetNames) *adminv1.RateLimitPolicy {
	disabled := !policy.Spec.Enabled && biz.ConfigurationApplied(policy.Generation, policy.Status.Conditions)
	status := policyStatus(policy.Generation, policy.Spec.Enabled, len(policy.Spec.TargetRefs), policy.Status.Conditions)
	reply := &adminv1.RateLimitPolicy{
		Id: policy.Name, Version: strconv.FormatInt(policy.Generation, 10), Status: resourceStatus(status),
		Name: policy.Spec.DisplayName, Description: policy.Spec.Description, Enabled: policy.Spec.Enabled,
		Targets:       policyTargets(policy.Generation, disabled, policy.Spec.TargetRefs, policy.Status.Targets, names),
		FailurePolicy: string(policy.Spec.FailurePolicy), CreatedAt: timestamp(policy.CreationTimestamp.Time),
		Response: &adminv1.RateLimitResponse{
			StatusCode: int32(policy.Spec.Response.StatusCode), Message: policy.Spec.Response.Message,
			QuotaHeaderEnabled: policy.Spec.Response.QuotaHeaderEnabled,
		},
	}
	if reply.FailurePolicy == "" {
		reply.FailurePolicy = string(resource.RateLimitFailurePolicyFailOpen)
	}
	if reply.Response.StatusCode == 0 {
		reply.Response.StatusCode = 429
	}
	if reply.Response.Message == "" {
		reply.Response.Message = "Too many requests"
	}
	for _, item := range policy.Spec.Rules {
		rule := &adminv1.RateLimitRule{Name: item.Name, Key: &adminv1.RateLimitKey{}, Limit: &adminv1.RateLimitQuota{
			Requests: int32(item.Limit.Requests), WindowSeconds: int32(item.Limit.WindowSeconds), Burst: int32(item.Limit.Burst),
		}}
		for _, part := range item.Key.Parts {
			rule.Key.Parts = append(rule.Key.Parts, &adminv1.RateLimitKeyPart{Type: string(part.Type), Name: part.Name})
		}
		reply.Rules = append(reply.Rules, rule)
	}
	return reply
}

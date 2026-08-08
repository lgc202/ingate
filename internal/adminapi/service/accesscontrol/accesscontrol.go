// Package accesscontrol 实现访问控制策略管理 API
package accesscontrol

import (
	"context"
	"net/netip"
	"strconv"
	"strings"

	"github.com/google/wire"
	"google.golang.org/protobuf/types/known/emptypb"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	accesscontrolbiz "github.com/lgc202/ingate/internal/adminapi/biz/accesscontrol"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// ProviderSet 提供访问控制策略协议服务
var ProviderSet = wire.NewSet(NewService)

// Service 实现访问控制策略管理 API
type Service struct {
	usecase *accesscontrolbiz.Usecase
}

// NewService 创建访问控制策略协议服务
func NewService(usecase *accesscontrolbiz.Usecase) *Service {
	return &Service{usecase: usecase}
}

func (s *Service) ListAccessControlPolicies(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListAccessControlPoliciesReply, error) {
	result, err := s.usecase.List(ctx)
	if err != nil {
		return nil, adminservice.OperationError(err, "查询访问控制策略失败")
	}
	reply := &adminv1.ListAccessControlPoliciesReply{Policies: make([]*adminv1.AccessControlPolicy, 0, len(result.Policies))}
	for i := range result.Policies {
		reply.Policies = append(reply.Policies, accessControlPolicyReply(&result.Policies[i], result.TargetNames))
	}
	return reply, nil
}

func (s *Service) GetAccessControlPolicy(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.AccessControlPolicy, error) {
	result, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, adminservice.OperationError(err, "查询访问控制策略失败")
	}
	return accessControlPolicyReply(result.Policy, result.TargetNames), nil
}

func (s *Service) CreateAccessControlPolicy(ctx context.Context, request *adminv1.CreateAccessControlPolicyRequest) (*adminv1.MutationReply, error) {
	spec, err := accessControlPolicySpec(
		request.GetName(), request.GetDescription(), request.GetEnabled(), request.GetTargets(),
		request.GetDefaultAction(), request.GetRules(), request.GetResponse(),
	)
	if err != nil {
		return nil, err
	}
	id, err := s.usecase.Create(ctx, spec)
	if err != nil {
		return nil, adminservice.OperationError(err, "创建访问控制策略失败")
	}
	return &adminv1.MutationReply{Success: true, Id: id}, nil
}

func (s *Service) UpdateAccessControlPolicy(ctx context.Context, request *adminv1.UpdateAccessControlPolicyRequest) (*adminv1.MutationReply, error) {
	spec, err := accessControlPolicySpec(
		request.GetName(), request.GetDescription(), request.GetEnabled(), request.GetTargets(),
		request.GetDefaultAction(), request.GetRules(), request.GetResponse(),
	)
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec); err != nil {
		return nil, adminservice.OperationError(err, "更新访问控制策略失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) SetAccessControlPolicyEnabled(ctx context.Context, request *adminv1.SetEnabledRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.SetEnabled(ctx, request.GetId(), request.GetEnabled()); err != nil {
		return nil, adminservice.OperationError(err, "更新访问控制策略状态失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) DeleteAccessControlPolicy(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, adminservice.OperationError(err, "删除访问控制策略失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func accessControlPolicySpec(
	name, description string,
	enabled bool,
	targets []*adminv1.PolicyTargetRef,
	defaultAction string,
	rules []*adminv1.AccessControlRule,
	response *adminv1.AccessControlDenyResponse,
) (resource.AccessControlPolicySpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.AccessControlPolicySpec{}, adminservice.BadRequest("名称不能为空")
	}
	refs, err := adminservice.PolicyTargetRefs(targets)
	if err != nil {
		return resource.AccessControlPolicySpec{}, err
	}
	action := resource.AccessControlAction(defaultAction)
	if action == "" {
		action = resource.AccessControlActionAllow
	}
	if action != resource.AccessControlActionAllow && action != resource.AccessControlActionDeny {
		return resource.AccessControlPolicySpec{}, adminservice.BadRequest("默认动作不正确")
	}
	if len(rules) == 0 && action != resource.AccessControlActionDeny {
		return resource.AccessControlPolicySpec{}, adminservice.BadRequest("至少需要一条访问控制规则，或将默认动作设置为拒绝")
	}
	spec := resource.AccessControlPolicySpec{
		DisplayName: name, Description: description, Enabled: enabled, TargetRefs: refs, DefaultAction: action,
	}
	seen := make(map[string]struct{}, len(rules))
	for _, input := range rules {
		if input == nil || strings.TrimSpace(input.GetName()) == "" {
			return resource.AccessControlPolicySpec{}, adminservice.BadRequest("访问控制规则名称不能为空")
		}
		rule := resource.AccessControlRule{Name: strings.TrimSpace(input.GetName()), Action: resource.AccessControlAction(input.GetAction())}
		if _, exists := seen[rule.Name]; exists {
			return resource.AccessControlPolicySpec{}, adminservice.BadRequest("访问控制规则名称不能重复")
		}
		seen[rule.Name] = struct{}{}
		if rule.Action != resource.AccessControlActionAllow && rule.Action != resource.AccessControlActionDeny {
			return resource.AccessControlPolicySpec{}, adminservice.BadRequest("访问控制规则动作不正确")
		}
		for _, inputCondition := range input.GetConditions() {
			if inputCondition == nil || strings.TrimSpace(inputCondition.GetValue()) == "" {
				return resource.AccessControlPolicySpec{}, adminservice.BadRequest("访问控制条件值不能为空")
			}
			condition := resource.AccessControlCondition{
				Type:  resource.AccessControlConditionType(inputCondition.GetType()),
				Name:  strings.TrimSpace(inputCondition.GetName()),
				Value: strings.TrimSpace(inputCondition.GetValue()),
			}
			switch condition.Type {
			case resource.AccessControlConditionTypeIP:
				condition.Name = ""
				if _, err := netip.ParseAddr(condition.Value); err != nil {
					if _, err := netip.ParsePrefix(condition.Value); err != nil {
						return resource.AccessControlPolicySpec{}, adminservice.BadRequest("客户端 IP 必须是 IP 地址或 CIDR")
					}
				}
			case resource.AccessControlConditionTypeHeader:
				if condition.Name == "" {
					return resource.AccessControlPolicySpec{}, adminservice.BadRequest("请求头访问控制条件必须填写名称")
				}
				if len(k8svalidation.IsHTTPHeaderName(condition.Name)) > 0 {
					return resource.AccessControlPolicySpec{}, adminservice.BadRequest("请求头名称不正确")
				}
			default:
				return resource.AccessControlPolicySpec{}, adminservice.BadRequest("访问控制条件类型不正确")
			}
			rule.Conditions = append(rule.Conditions, condition)
		}
		spec.Rules = append(spec.Rules, rule)
	}
	if response != nil {
		if response.GetStatusCode() != 0 && (response.GetStatusCode() < 400 || response.GetStatusCode() > 599) {
			return resource.AccessControlPolicySpec{}, adminservice.BadRequest("拒绝响应状态码必须在 400 到 599 之间")
		}
		spec.Response = resource.AccessControlDenyResponse{
			StatusCode: int(response.GetStatusCode()), Message: response.GetMessage(),
		}
	}
	return spec, nil
}

func accessControlPolicyReply(policy *resource.AccessControlPolicy, names biz.PolicyTargetNames) *adminv1.AccessControlPolicy {
	status := biz.PolicyStatus(policy.Generation, policy.Spec.Enabled, len(policy.Spec.TargetRefs), policy.Status.Conditions)
	disabled := status.State == biz.ResourceStateDisabled
	reply := &adminv1.AccessControlPolicy{
		Id: policy.Name, Version: strconv.FormatInt(policy.Generation, 10), Status: adminservice.ResourceStatus(status),
		Name: policy.Spec.DisplayName, Description: policy.Spec.Description, Enabled: policy.Spec.Enabled,
		Targets:       adminservice.PolicyTargets(policy.Generation, disabled, policy.Spec.TargetRefs, policy.Status.Targets, names),
		DefaultAction: string(policy.Spec.DefaultAction), CreatedAt: adminservice.Timestamp(policy.CreationTimestamp.Time),
		Response: &adminv1.AccessControlDenyResponse{
			StatusCode: int32(policy.Spec.Response.StatusCode), Message: policy.Spec.Response.Message,
		},
	}
	if reply.DefaultAction == "" {
		reply.DefaultAction = string(resource.AccessControlActionAllow)
	}
	if reply.Response.StatusCode == 0 {
		reply.Response.StatusCode = 403
	}
	if reply.Response.Message == "" {
		reply.Response.Message = "Access denied"
	}
	for _, item := range policy.Spec.Rules {
		rule := &adminv1.AccessControlRule{Name: item.Name, Action: string(item.Action)}
		for _, condition := range item.Conditions {
			rule.Conditions = append(rule.Conditions, &adminv1.AccessControlCondition{
				Type: string(condition.Type), Name: condition.Name, Value: condition.Value,
			})
		}
		reply.Rules = append(reply.Rules, rule)
	}
	return reply
}

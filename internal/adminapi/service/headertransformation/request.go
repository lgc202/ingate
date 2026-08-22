package headertransformation

import (
	"strings"

	"golang.org/x/net/http/httpguts"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func createSpec(request *adminv1.CreateHeaderTransformationPolicyRequest) (resource.HeaderTransformationPolicySpec, error) {
	return policySpec(
		request.GetName(),
		true,
		request.GetTargets(),
		request.GetRequestRules(),
		request.GetResponseRules(),
	)
}

func updateSpec(request *adminv1.UpdateHeaderTransformationPolicyRequest) (resource.HeaderTransformationPolicySpec, error) {
	return policySpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetTargets(),
		request.GetRequestRules(),
		request.GetResponseRules(),
	)
}

func policySpec(
	name string,
	enabled bool,
	targets []*adminv1.PolicyTargetRef,
	requestRules []*adminv1.HeaderTransformationRule,
	responseRules []*adminv1.HeaderTransformationRule,
) (resource.HeaderTransformationPolicySpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.HeaderTransformationPolicySpec{}, adminservice.BadRequest("请求响应转换策略名称不能为空")
	}
	targetRefs, err := adminservice.PolicyTargetRefs(targets, resource.KindRoute)
	if err != nil {
		return resource.HeaderTransformationPolicySpec{}, err
	}
	requests, err := transformationRules(requestRules)
	if err != nil {
		return resource.HeaderTransformationPolicySpec{}, err
	}
	responses, err := transformationRules(responseRules)
	if err != nil {
		return resource.HeaderTransformationPolicySpec{}, err
	}
	if len(requests)+len(responses) == 0 {
		return resource.HeaderTransformationPolicySpec{}, adminservice.BadRequest("请至少配置一条请求或响应 Header 规则")
	}
	return resource.HeaderTransformationPolicySpec{
		DisplayName:   name,
		Enabled:       enabled,
		TargetRefs:    targetRefs,
		RequestRules:  requests,
		ResponseRules: responses,
	}, nil
}

func transformationRules(values []*adminv1.HeaderTransformationRule) ([]resource.HeaderTransformationRule, error) {
	rules := make([]resource.HeaderTransformationRule, 0, len(values))
	for _, value := range values {
		operation, err := transformationOperation(value.GetOperation())
		if err != nil {
			return nil, err
		}
		name := strings.ToLower(strings.TrimSpace(value.GetName()))
		if !validHeaderName(name) {
			return nil, adminservice.BadRequest("Header 名称格式不正确")
		}
		rule := resource.HeaderTransformationRule{
			Operation: operation,
			Name:      name,
			Value:     strings.TrimSpace(value.GetValue()),
		}
		if operation == resource.HeaderTransformationRename {
			rule.Value = strings.ToLower(rule.Value)
			if !validHeaderName(rule.Value) {
				return nil, adminservice.BadRequest("重命名规则的新 Header 名称格式不正确")
			}
		}
		if operation == resource.HeaderTransformationRemove {
			rule.Value = ""
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func transformationOperation(value adminv1.HeaderTransformationOperation) (resource.HeaderTransformationOperation, error) {
	switch value {
	case adminv1.HeaderTransformationOperation_HEADER_TRANSFORMATION_OPERATION_REMOVE:
		return resource.HeaderTransformationRemove, nil
	case adminv1.HeaderTransformationOperation_HEADER_TRANSFORMATION_OPERATION_RENAME:
		return resource.HeaderTransformationRename, nil
	case adminv1.HeaderTransformationOperation_HEADER_TRANSFORMATION_OPERATION_REPLACE:
		return resource.HeaderTransformationReplace, nil
	case adminv1.HeaderTransformationOperation_HEADER_TRANSFORMATION_OPERATION_ADD:
		return resource.HeaderTransformationAdd, nil
	case adminv1.HeaderTransformationOperation_HEADER_TRANSFORMATION_OPERATION_APPEND:
		return resource.HeaderTransformationAppend, nil
	default:
		return "", adminservice.BadRequest("Header 转换操作不正确")
	}
}

func validHeaderName(value string) bool {
	return value != "" && !strings.HasPrefix(value, ":") && httpguts.ValidHeaderFieldName(value)
}

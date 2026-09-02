package headertransformation

import (
	"fmt"
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/headertransformationconfig"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
)

func parseHeaderTransformationPolicySpec(
	displayName string,
	enabled bool,
	targetConfigs []*adminv1.PolicyTargetRef,
	requestRuleConfigs []*adminv1.HeaderTransformationRule,
	responseRuleConfigs []*adminv1.HeaderTransformationRule,
) (resource.HeaderTransformationPolicySpec, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return resource.HeaderTransformationPolicySpec{}, adminv1.ErrorInvalidArgument("请求响应转换策略名称不能为空")
	}
	targets, err := adminservice.PolicyTargetRefs(targetConfigs, resource.KindRoute)
	if err != nil {
		return resource.HeaderTransformationPolicySpec{}, err
	}
	ruleCount := len(requestRuleConfigs) + len(responseRuleConfigs)
	if ruleCount == 0 {
		return resource.HeaderTransformationPolicySpec{}, adminv1.ErrorInvalidArgument("请至少配置一条请求或响应 Header 规则")
	}
	if ruleCount > headertransformationconfig.MaxRules {
		return resource.HeaderTransformationPolicySpec{}, adminv1.ErrorInvalidArgument("请求和响应 Header 规则总数超过限制")
	}
	requestRules, err := parseHeaderTransformationRules(requestRuleConfigs, "请求")
	if err != nil {
		return resource.HeaderTransformationPolicySpec{}, err
	}
	responseRules, err := parseHeaderTransformationRules(responseRuleConfigs, "响应")
	if err != nil {
		return resource.HeaderTransformationPolicySpec{}, err
	}

	return resource.HeaderTransformationPolicySpec{
		DisplayName:   displayName,
		Enabled:       enabled,
		TargetRefs:    targets,
		RequestRules:  requestRules,
		ResponseRules: responseRules,
	}, nil
}

func parseHeaderTransformationRules(
	configs []*adminv1.HeaderTransformationRule,
	direction string,
) ([]resource.HeaderTransformationRule, error) {
	rules := make([]resource.HeaderTransformationRule, len(configs))
	for i, config := range configs {
		if config == nil {
			return nil, adminv1.ErrorInvalidArgument("%s", fmt.Sprintf("第 %d 条%s Header 规则不能为空", i+1, direction))
		}
		operation, err := parseHeaderTransformationOperation(config.GetOperation())
		if err != nil {
			return nil, err
		}
		headerName := httpheader.NormalizeName(config.GetName())
		if !httpheader.IsValidName(headerName) {
			return nil, adminv1.ErrorInvalidArgument("%s", fmt.Sprintf("第 %d 条%s Header 规则的名称格式不正确", i+1, direction))
		}
		headerValue, err := parseHeaderTransformationValue(
			operation,
			config.GetValue(),
			i,
			direction,
		)
		if err != nil {
			return nil, err
		}
		rules[i] = resource.HeaderTransformationRule{
			Operation: operation,
			Name:      headerName,
			Value:     headerValue,
		}
	}
	return rules, nil
}

func parseHeaderTransformationOperation(
	operation adminv1.HeaderTransformationOperation,
) (resource.HeaderTransformationOperation, error) {
	switch operation {
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
		return "", adminv1.ErrorInvalidArgument("Header 转换操作不正确")
	}
}

func parseHeaderTransformationValue(
	operation resource.HeaderTransformationOperation,
	value string,
	ruleIndex int,
	direction string,
) (string, error) {
	value = httpheader.NormalizeValue(value)
	switch operation {
	case resource.HeaderTransformationRemove:
		if value != "" {
			return "", adminv1.ErrorInvalidArgument("%s", fmt.Sprintf("第 %d 条%s Header 删除规则不能配置值", ruleIndex+1, direction))
		}
		return "", nil
	case resource.HeaderTransformationRename:
		value = httpheader.NormalizeName(value)
		if !httpheader.IsValidName(value) {
			return "", adminv1.ErrorInvalidArgument("%s", fmt.Sprintf("第 %d 条%s Header 重命名规则的新名称格式不正确", ruleIndex+1, direction))
		}
		return value, nil
	case resource.HeaderTransformationReplace,
		resource.HeaderTransformationAdd,
		resource.HeaderTransformationAppend:
		if !httpheader.IsValidValue(value) {
			return "", adminv1.ErrorInvalidArgument("%s", fmt.Sprintf("第 %d 条%s Header 规则的值格式不正确", ruleIndex+1, direction))
		}
		return value, nil
	default:
		return "", adminv1.ErrorInvalidArgument("Header 转换操作不正确")
	}
}

package mockresponse

import (
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/service/policy/target"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	apivalidation "github.com/lgc202/ingate/internal/pkg/apis/gateway/validation"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
)

func parseSpec(
	displayName string,
	enabled bool,
	targetConfigs []*adminv1.PolicyTargetRef,
	statusCode int32,
	contentType string,
	headerConfigs []*adminv1.MockResponseHeader,
	body string,
) (resource.MockResponsePolicySpec, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return resource.MockResponsePolicySpec{}, adminv1.ErrorInvalidArgument("模拟响应策略名称不能为空")
	}
	targets, err := target.Refs(targetConfigs, resource.KindRoute)
	if err != nil {
		return resource.MockResponsePolicySpec{}, err
	}
	if !apivalidation.IsValidStatusCode(statusCode) {
		return resource.MockResponsePolicySpec{}, adminv1.ErrorInvalidArgument("响应状态码必须在 200 到 599 之间")
	}
	contentType, valid := apivalidation.NormalizeContentType(contentType)
	if !valid {
		return resource.MockResponsePolicySpec{}, adminv1.ErrorInvalidArgument("响应内容类型格式不正确")
	}
	if len(body) > apivalidation.MaxBodyBytes {
		return resource.MockResponsePolicySpec{}, adminv1.ErrorInvalidArgument("响应正文不能超过 1 MiB")
	}
	headers, err := parseHeaders(headerConfigs)
	if err != nil {
		return resource.MockResponsePolicySpec{}, err
	}

	return resource.MockResponsePolicySpec{
		DisplayName: displayName,
		Enabled:     enabled,
		TargetRefs:  targets,
		StatusCode:  statusCode,
		ContentType: contentType,
		Headers:     headers,
		Body:        body,
	}, nil
}

func parseHeaders(
	configs []*adminv1.MockResponseHeader,
) ([]resource.HeaderValue, error) {
	if len(configs) > apivalidation.MaxHeaders {
		return nil, adminv1.ErrorInvalidArgument("响应 Header 数量超过限制")
	}

	headers := make([]resource.HeaderValue, len(configs))
	seen := make(map[string]bool, len(configs))
	for i, config := range configs {
		if config == nil {
			return nil, adminv1.ErrorInvalidArgument("第 %d 条响应 Header 不能为空", i+1)
		}
		name := httpheader.NormalizeName(config.GetName())
		if !httpheader.IsValidName(name) {
			return nil, adminv1.ErrorInvalidArgument("第 %d 条响应 Header 名称格式不正确", i+1)
		}
		if apivalidation.IsReservedHeaderName(name) {
			return nil, adminv1.ErrorInvalidArgument("响应 Header %q 由系统管理，不能自行配置", name)
		}
		if seen[name] {
			return nil, adminv1.ErrorInvalidArgument("响应 Header %q 不能重复", name)
		}
		value := httpheader.NormalizeValue(config.GetValue())
		if !httpheader.IsValidValue(value) {
			return nil, adminv1.ErrorInvalidArgument("响应 Header %q 的值格式不正确", name)
		}
		seen[name] = true
		headers[i] = resource.HeaderValue{Name: name, Value: value}
	}
	return headers, nil
}

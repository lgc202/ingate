package mockresponse

import (
	"mime"
	"strings"

	"golang.org/x/net/http/httpguts"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

const maxResponseBodyBytes = 1 << 20

func createSpec(request *adminv1.CreateMockResponsePolicyRequest) (resource.MockResponsePolicySpec, error) {
	return policySpec(
		request.GetName(),
		true,
		request.GetTargets(),
		request.GetStatusCode(),
		request.GetContentType(),
		request.GetHeaders(),
		request.GetBody(),
	)
}

func updateSpec(request *adminv1.UpdateMockResponsePolicyRequest) (resource.MockResponsePolicySpec, error) {
	return policySpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetTargets(),
		request.GetStatusCode(),
		request.GetContentType(),
		request.GetHeaders(),
		request.GetBody(),
	)
}

func policySpec(
	name string,
	enabled bool,
	targets []*adminv1.PolicyTargetRef,
	statusCode int32,
	contentType string,
	headers []*adminv1.MockResponseHeader,
	body string,
) (resource.MockResponsePolicySpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.MockResponsePolicySpec{}, adminservice.BadRequest("模拟响应策略名称不能为空")
	}
	targetRefs, err := adminservice.PolicyTargetRefs(targets, resource.KindRoute)
	if err != nil {
		return resource.MockResponsePolicySpec{}, err
	}
	if statusCode < 200 || statusCode > 599 {
		return resource.MockResponsePolicySpec{}, adminservice.BadRequest("响应状态码必须在 200 到 599 之间")
	}
	contentType = strings.TrimSpace(contentType)
	if _, _, err := mime.ParseMediaType(contentType); err != nil {
		return resource.MockResponsePolicySpec{}, adminservice.BadRequest("响应内容类型格式不正确")
	}
	if len(body) > maxResponseBodyBytes {
		return resource.MockResponsePolicySpec{}, adminservice.BadRequest("响应正文不能超过 1 MiB")
	}
	responseHeaders, err := headerValues(headers)
	if err != nil {
		return resource.MockResponsePolicySpec{}, err
	}
	return resource.MockResponsePolicySpec{
		DisplayName: name,
		Enabled:     enabled,
		TargetRefs:  targetRefs,
		StatusCode:  statusCode,
		ContentType: contentType,
		Headers:     responseHeaders,
		Body:        body,
	}, nil
}

func headerValues(values []*adminv1.MockResponseHeader) ([]resource.HeaderValue, error) {
	headers := make([]resource.HeaderValue, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		name := strings.ToLower(strings.TrimSpace(value.GetName()))
		if name == "" || strings.HasPrefix(name, ":") || !httpguts.ValidHeaderFieldName(name) {
			return nil, adminservice.BadRequest("响应 Header 名称格式不正确")
		}
		if name == "content-type" {
			return nil, adminservice.BadRequest("Content-Type 请通过响应内容类型配置")
		}
		if seen[name] {
			return nil, adminservice.BadRequest("响应 Header 名称不能重复")
		}
		seen[name] = true
		headerValue := strings.TrimSpace(value.GetValue())
		if !httpguts.ValidHeaderFieldValue(headerValue) {
			return nil, adminservice.BadRequest("响应 Header 值包含非法字符")
		}
		headers = append(headers, resource.HeaderValue{Name: name, Value: headerValue})
	}
	return headers, nil
}

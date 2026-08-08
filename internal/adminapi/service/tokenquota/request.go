package tokenquota

import (
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func buildTokenQuotaPolicySpec(
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
		return resource.TokenQuotaPolicySpec{}, adminservice.BadRequest("名称不能为空")
	}
	refs, err := adminservice.BuildPolicyTargetRefs(targets)
	if err != nil {
		return resource.TokenQuotaPolicySpec{}, err
	}
	if subject == nil || quota == nil {
		return resource.TokenQuotaPolicySpec{}, adminservice.BadRequest("额度划分方式和 Token 额度不能为空")
	}
	subjectValue := resource.TokenQuotaSubject{
		Type: resource.TokenQuotaSubjectType(subject.GetType()), HeaderName: strings.TrimSpace(subject.GetHeaderName()),
	}
	switch subjectValue.Type {
	case resource.TokenQuotaSubjectTypeShared, resource.TokenQuotaSubjectTypeIP:
		subjectValue.HeaderName = ""
	case resource.TokenQuotaSubjectTypeHeader:
		if subjectValue.HeaderName == "" || len(k8svalidation.IsHTTPHeaderName(subjectValue.HeaderName)) > 0 {
			return resource.TokenQuotaPolicySpec{}, adminservice.BadRequest("请求头名称不正确")
		}
	default:
		return resource.TokenQuotaPolicySpec{}, adminservice.BadRequest("额度划分方式不正确")
	}
	if quota.GetTokens() <= 0 || quota.GetTokens() > resource.TokenQuotaMaxTokens {
		return resource.TokenQuotaPolicySpec{}, adminservice.BadRequest("Token 额度超出支持范围")
	}
	if quota.GetWindowSeconds() <= 0 || quota.GetWindowSeconds() > resource.TokenQuotaMaxWindowSeconds {
		return resource.TokenQuotaPolicySpec{}, adminservice.BadRequest("统计周期超出支持范围")
	}
	policy := resource.TokenQuotaFailurePolicy(failurePolicy)
	if policy != resource.TokenQuotaFailurePolicyFailOpen && policy != resource.TokenQuotaFailurePolicyFailClose {
		return resource.TokenQuotaPolicySpec{}, adminservice.BadRequest("失败策略不正确")
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

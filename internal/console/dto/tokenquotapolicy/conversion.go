package tokenquotapolicy

import (
	consoledto "github.com/lgc202/ingate/internal/console/dto"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Spec 将已校验的创建请求转换为声明式 TokenQuotaPolicySpec
func (r CreateTokenQuotaPolicyReq) Spec() resource.TokenQuotaPolicySpec {
	return r.TokenQuotaPolicyConfig.spec()
}

// Spec 将已校验的更新请求转换为声明式 TokenQuotaPolicySpec
func (r UpdateTokenQuotaPolicyReq) Spec() resource.TokenQuotaPolicySpec {
	return r.TokenQuotaPolicyConfig.spec()
}

func (c TokenQuotaPolicyConfig) spec() resource.TokenQuotaPolicySpec {
	return resource.TokenQuotaPolicySpec{
		DisplayName: c.Name,
		Description: c.Description,
		Enabled:     c.Enabled,
		TargetRefs:  tokenQuotaTargetRefsFromRequest(c.Targets),
		Subject: resource.TokenQuotaSubject{
			Type:       resource.TokenQuotaSubjectType(c.Subject.Type),
			HeaderName: c.Subject.HeaderName,
		},
		Quota: resource.TokenQuota{
			Tokens:        c.Quota.Tokens,
			WindowSeconds: c.Quota.WindowSeconds,
		},
		FailurePolicy: resource.TokenQuotaFailurePolicy(c.FailurePolicy),
		Response: resource.TokenQuotaResponse{
			Message: c.Response.Message,
		},
	}
}

func tokenQuotaTargetRefsFromRequest(targets []consoledto.PolicyTargetReq) []resource.PolicyTargetRef {
	refs := make([]resource.PolicyTargetRef, 0, len(targets))
	for _, target := range targets {
		refs = append(refs, resource.PolicyTargetRef{
			Kind: resource.Kind(target.Kind),
			Name: target.ID,
		})
	}
	return refs
}

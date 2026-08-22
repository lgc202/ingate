package tokenquota

import (
	"strings"
	"time"
	_ "time/tzdata"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func createSpec(request *adminv1.CreateTokenQuotaPolicyRequest) (resource.TokenQuotaPolicySpec, error) {
	return tokenQuotaSpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetTargets(),
		request.GetTimeZone(),
		request.GetLimits(),
	)
}

func updateSpec(request *adminv1.UpdateTokenQuotaPolicyRequest) (resource.TokenQuotaPolicySpec, error) {
	return tokenQuotaSpec(
		request.GetName(),
		request.GetEnabled(),
		request.GetTargets(),
		request.GetTimeZone(),
		request.GetLimits(),
	)
}

func tokenQuotaSpec(
	name string,
	enabled bool,
	targets []*adminv1.PolicyTargetRef,
	timeZone string,
	limits []*adminv1.TokenQuotaLimit,
) (resource.TokenQuotaPolicySpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.TokenQuotaPolicySpec{}, adminservice.BadRequest("Token 额度策略名称不能为空")
	}
	refs, err := adminservice.PolicyTargetRefs(targets, resource.KindCaller)
	if err != nil {
		return resource.TokenQuotaPolicySpec{}, err
	}
	timeZone = strings.TrimSpace(timeZone)
	if _, err := time.LoadLocation(timeZone); err != nil {
		return resource.TokenQuotaPolicySpec{}, adminservice.BadRequest("额度周期时区不正确")
	}
	converted, err := tokenQuotaLimits(limits)
	if err != nil {
		return resource.TokenQuotaPolicySpec{}, err
	}
	return resource.TokenQuotaPolicySpec{
		DisplayName: name,
		Enabled:     enabled,
		TargetRefs:  refs,
		TimeZone:    timeZone,
		Limits:      converted,
	}, nil
}

func tokenQuotaLimits(limits []*adminv1.TokenQuotaLimit) ([]resource.TokenQuotaLimit, error) {
	converted := make([]resource.TokenQuotaLimit, 0, len(limits))
	seen := make(map[resource.TokenQuotaPeriod]bool, len(limits))
	for _, limit := range limits {
		if limit == nil {
			return nil, adminservice.BadRequest("Token 额度不能为空")
		}
		period, err := tokenQuotaPeriod(limit.GetPeriod())
		if err != nil {
			return nil, err
		}
		if seen[period] {
			return nil, adminservice.BadRequest("同一额度周期不能重复配置")
		}
		seen[period] = true
		converted = append(converted, resource.TokenQuotaLimit{Period: period, Tokens: limit.GetTokens()})
	}
	return converted, nil
}

func tokenQuotaPeriod(period adminv1.TokenQuotaPeriod) (resource.TokenQuotaPeriod, error) {
	switch period {
	case adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_DAY:
		return resource.TokenQuotaPeriodDay, nil
	case adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_WEEK:
		return resource.TokenQuotaPeriodWeek, nil
	case adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_MONTH:
		return resource.TokenQuotaPeriodMonth, nil
	default:
		return "", adminservice.BadRequest("Token 额度周期不正确")
	}
}

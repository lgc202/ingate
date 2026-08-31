package tokenquota

import (
	"slices"
	"strings"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/tokenquotaconfig"
)

func parseTokenQuotaPolicySpec(
	displayName string,
	enabled bool,
	targetConfigs []*adminv1.PolicyTargetRef,
	timeZone string,
	limitConfigs []*adminv1.TokenQuotaLimit,
) (resource.TokenQuotaPolicySpec, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return resource.TokenQuotaPolicySpec{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"Token 额度策略名称不能为空",
		)
	}
	targets, err := adminservice.PolicyTargetRefs(targetConfigs, resource.KindCaller)
	if err != nil {
		return resource.TokenQuotaPolicySpec{}, err
	}
	timeZone, _, valid := tokenquotaconfig.LoadLocation(timeZone)
	if !valid {
		return resource.TokenQuotaPolicySpec{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"额度周期时区不正确",
		)
	}
	limits, err := parseTokenQuotaLimits(limitConfigs)
	if err != nil {
		return resource.TokenQuotaPolicySpec{}, err
	}

	return resource.TokenQuotaPolicySpec{
		DisplayName: displayName,
		Enabled:     enabled,
		TargetRefs:  targets,
		TimeZone:    timeZone,
		Limits:      limits,
	}, nil
}

func parseTokenQuotaLimits(
	configs []*adminv1.TokenQuotaLimit,
) ([]resource.TokenQuotaLimit, error) {
	if len(configs) == 0 {
		return nil, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"请至少配置一项 Token 额度",
		)
	}
	if len(configs) > tokenquotaconfig.MaxLimits {
		return nil, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"Token 额度周期数量超过限制",
		)
	}

	limits := make([]resource.TokenQuotaLimit, len(configs))
	seen := make(map[resource.TokenQuotaPeriod]bool, len(configs))
	for i, config := range configs {
		if config == nil {
			return nil, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"Token 额度不能为空",
			)
		}
		period, err := parseTokenQuotaPeriod(config.GetPeriod())
		if err != nil {
			return nil, err
		}
		if seen[period] {
			return nil, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"同一额度周期不能重复配置",
			)
		}
		tokens := config.GetTokens()
		if !tokenquotaconfig.IsValidTokenLimit(tokens) {
			return nil, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"Token 额度超出支持范围",
			)
		}
		seen[period] = true
		limits[i] = resource.TokenQuotaLimit{Period: period, Tokens: tokens}
	}
	slices.SortFunc(limits, func(a, b resource.TokenQuotaLimit) int {
		return tokenQuotaPeriodOrder(a.Period) - tokenQuotaPeriodOrder(b.Period)
	})
	return limits, nil
}

func parseTokenQuotaPeriod(
	period adminv1.TokenQuotaPeriod,
) (resource.TokenQuotaPeriod, error) {
	switch period {
	case adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_DAY:
		return resource.TokenQuotaPeriodDay, nil
	case adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_WEEK:
		return resource.TokenQuotaPeriodWeek, nil
	case adminv1.TokenQuotaPeriod_TOKEN_QUOTA_PERIOD_MONTH:
		return resource.TokenQuotaPeriodMonth, nil
	default:
		return "", errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"Token 额度周期不正确",
		)
	}
}

func tokenQuotaPeriodOrder(period resource.TokenQuotaPeriod) int {
	switch period {
	case resource.TokenQuotaPeriodDay:
		return 1
	case resource.TokenQuotaPeriodWeek:
		return 2
	case resource.TokenQuotaPeriodMonth:
		return 3
	default:
		return 4
	}
}

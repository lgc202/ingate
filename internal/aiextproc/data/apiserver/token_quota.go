package apiserver

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"k8s.io/client-go/tools/cache"

	"github.com/lgc202/ingate/internal/aiextproc/biz/tokenquota"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/policyconfig"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/tokenquotaconfig"
)

type compiledTokenQuotaPolicy struct {
	policy    tokenquota.Policy
	callerIDs []string
}

// ActivePolicies 返回当前调用方命中的全部已启用 Token 额度策略。
func (c *ConfigCache) ActivePolicies(callerID string) ([]tokenquota.Policy, error) {
	if !c.ready.Load() {
		return nil, errors.New("AI execution config cache is not ready")
	}

	c.tokenQuotaMu.RLock()
	indexed := c.policiesByCaller[callerID]
	if len(indexed) > tokenquotaconfig.MaxPoliciesPerCaller {
		c.tokenQuotaMu.RUnlock()
		return nil, fmt.Errorf(
			"caller %q matches %d token quota policies; limit is %d",
			callerID,
			len(indexed),
			tokenquotaconfig.MaxPoliciesPerCaller,
		)
	}
	policies := make([]tokenquota.Policy, 0, len(indexed))
	for _, policy := range indexed {
		policy.Limits = slices.Clone(policy.Limits)
		policies = append(policies, policy)
	}
	c.tokenQuotaMu.RUnlock()

	slices.SortFunc(policies, func(a, b tokenquota.Policy) int {
		return cmp.Compare(a.ID, b.ID)
	})
	return policies, nil
}

func (c *ConfigCache) upsertTokenQuotaPolicy(object any) {
	policy, ok := object.(*resource.TokenQuotaPolicy)
	if !ok {
		c.logger.Error(
			"ignore unexpected TokenQuotaPolicy informer object",
			"object_type",
			fmt.Sprintf("%T", object),
		)
		return
	}
	if !policy.Spec.Enabled {
		c.removeTokenQuotaPolicy(policy.Name)
		return
	}

	compiled, err := compileTokenQuotaPolicy(policy)
	if err != nil {
		// 持久化事实一旦失效，不能继续执行上一版已经过期的额度规则。
		c.removeTokenQuotaPolicy(policy.Name)
		c.logger.Error(
			"ignore invalid TokenQuotaPolicy",
			"policy_id",
			policy.Name,
			"err",
			err,
		)
		return
	}
	c.storeTokenQuotaPolicy(compiled)
}

func (c *ConfigCache) deleteTokenQuotaPolicy(object any) {
	policy, ok := deletedTokenQuotaPolicy(object)
	if !ok {
		c.logger.Error(
			"ignore unexpected deleted TokenQuotaPolicy informer object",
			"object_type",
			fmt.Sprintf("%T", object),
		)
		return
	}
	c.removeTokenQuotaPolicy(policy.Name)
}

func (c *ConfigCache) storeTokenQuotaPolicy(compiled compiledTokenQuotaPolicy) {
	c.tokenQuotaMu.Lock()
	defer c.tokenQuotaMu.Unlock()

	c.removeTokenQuotaPolicyLocked(compiled.policy.ID)
	c.tokenQuotaPolicies[compiled.policy.ID] = compiled
	for _, callerID := range compiled.callerIDs {
		policies := c.policiesByCaller[callerID]
		if policies == nil {
			policies = make(map[string]tokenquota.Policy)
			c.policiesByCaller[callerID] = policies
		}
		policies[compiled.policy.ID] = compiled.policy
	}
}

func (c *ConfigCache) removeTokenQuotaPolicy(policyID string) {
	c.tokenQuotaMu.Lock()
	defer c.tokenQuotaMu.Unlock()
	c.removeTokenQuotaPolicyLocked(policyID)
}

func (c *ConfigCache) removeTokenQuotaPolicyLocked(policyID string) {
	compiled, exists := c.tokenQuotaPolicies[policyID]
	if !exists {
		return
	}
	delete(c.tokenQuotaPolicies, policyID)
	for _, callerID := range compiled.callerIDs {
		policies := c.policiesByCaller[callerID]
		delete(policies, policyID)
		if len(policies) == 0 {
			delete(c.policiesByCaller, callerID)
		}
	}
}

func compileTokenQuotaPolicy(policy *resource.TokenQuotaPolicy) (compiledTokenQuotaPolicy, error) {
	if !resourceconfig.IsCanonicalID(policy.Name) {
		return compiledTokenQuotaPolicy{}, errors.New("metadata.name must be a canonical UUID")
	}
	if policy.Spec.DisplayName == "" || strings.TrimSpace(policy.Spec.DisplayName) != policy.Spec.DisplayName {
		return compiledTokenQuotaPolicy{}, errors.New("displayName must be non-empty without surrounding whitespace")
	}
	timeZone, location, valid := tokenquotaconfig.LoadLocation(policy.Spec.TimeZone)
	if !valid || timeZone != policy.Spec.TimeZone {
		return compiledTokenQuotaPolicy{}, errors.New("timeZone must be a valid IANA time zone")
	}
	if len(policy.Spec.TargetRefs) > policyconfig.MaxTargets {
		return compiledTokenQuotaPolicy{}, fmt.Errorf(
			"target count exceeds %d",
			policyconfig.MaxTargets,
		)
	}
	if len(policy.Spec.Limits) == 0 || len(policy.Spec.Limits) > tokenquotaconfig.MaxLimits {
		return compiledTokenQuotaPolicy{}, fmt.Errorf(
			"limit count must be between 1 and %d",
			tokenquotaconfig.MaxLimits,
		)
	}

	callerIDs := make([]string, len(policy.Spec.TargetRefs))
	seenCallers := make(map[string]bool, len(policy.Spec.TargetRefs))
	for i, ref := range policy.Spec.TargetRefs {
		if ref.Kind != resource.KindCaller {
			return compiledTokenQuotaPolicy{}, fmt.Errorf("target %d has unsupported kind %q", i, ref.Kind)
		}
		callerID, valid := resourceconfig.NormalizeID(ref.Name)
		if !valid || callerID != ref.Name {
			return compiledTokenQuotaPolicy{}, fmt.Errorf("target %d has invalid Caller ID", i)
		}
		if seenCallers[callerID] {
			return compiledTokenQuotaPolicy{}, fmt.Errorf("target %d duplicates Caller %q", i, callerID)
		}
		seenCallers[callerID] = true
		callerIDs[i] = callerID
	}
	slices.Sort(callerIDs)

	limits := make([]tokenquota.Limit, len(policy.Spec.Limits))
	seenPeriods := make(map[tokenquota.Period]bool, len(policy.Spec.Limits))
	for i, limit := range policy.Spec.Limits {
		period, valid := tokenQuotaPeriod(limit.Period)
		if !valid {
			return compiledTokenQuotaPolicy{}, fmt.Errorf("limit %d has unsupported period %q", i, limit.Period)
		}
		if seenPeriods[period] {
			return compiledTokenQuotaPolicy{}, fmt.Errorf("limit %d duplicates period %q", i, period)
		}
		if !tokenquotaconfig.IsValidTokenLimit(limit.Tokens) {
			return compiledTokenQuotaPolicy{}, fmt.Errorf("limit %d has invalid token count", i)
		}
		seenPeriods[period] = true
		limits[i] = tokenquota.Limit{Period: period, Tokens: limit.Tokens}
	}
	slices.SortFunc(limits, func(a, b tokenquota.Limit) int {
		return tokenQuotaPeriodOrder(a.Period) - tokenQuotaPeriodOrder(b.Period)
	})

	return compiledTokenQuotaPolicy{
		policy: tokenquota.Policy{
			ID:       policy.Name,
			Name:     policy.Spec.DisplayName,
			TimeZone: location,
			Limits:   limits,
		},
		callerIDs: callerIDs,
	}, nil
}

func deletedTokenQuotaPolicy(object any) (*resource.TokenQuotaPolicy, bool) {
	if policy, ok := object.(*resource.TokenQuotaPolicy); ok {
		return policy, true
	}
	tombstone, ok := object.(cache.DeletedFinalStateUnknown)
	if !ok {
		return nil, false
	}
	policy, ok := tombstone.Obj.(*resource.TokenQuotaPolicy)
	return policy, ok
}

func tokenQuotaPeriod(period resource.TokenQuotaPeriod) (tokenquota.Period, bool) {
	switch period {
	case resource.TokenQuotaPeriodDay:
		return tokenquota.PeriodDay, true
	case resource.TokenQuotaPeriodWeek:
		return tokenquota.PeriodWeek, true
	case resource.TokenQuotaPeriodMonth:
		return tokenquota.PeriodMonth, true
	default:
		return "", false
	}
}

func tokenQuotaPeriodOrder(period tokenquota.Period) int {
	switch period {
	case tokenquota.PeriodDay:
		return 1
	case tokenquota.PeriodWeek:
		return 2
	case tokenquota.PeriodMonth:
		return 3
	default:
		return 4
	}
}

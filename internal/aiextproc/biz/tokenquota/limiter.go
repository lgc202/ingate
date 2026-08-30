// Package tokenquota 执行调用方模型 Token 额度检查和结算。
package tokenquota

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/tokenquotaconfig"
)

const (
	// PeriodDay 表示额度按策略时区的自然日重置。
	PeriodDay Period = "day"
	// PeriodWeek 表示额度按策略时区从周一开始的自然周重置。
	PeriodWeek Period = "week"
	// PeriodMonth 表示额度按策略时区的自然月重置。
	PeriodMonth Period = "month"
)

// Period 表示额度对应的自然周期。
type Period string

// Limit 定义一个自然周期内允许使用的总 Token 数。
type Limit struct {
	Period Period
	Tokens int64
}

// Policy 是请求执行阶段需要的最小 Token 额度配置。
type Policy struct {
	ID       string
	Name     string
	TimeZone *time.Location
	Limits   []Limit
}

// Bucket 标识一个调用方在某项策略和自然周期内的计数器。
type Bucket struct {
	CallerID   string
	PolicyID   string
	PolicyName string
	Period     Period
	Start      time.Time
	End        time.Time
	Limit      int64
}

// Usage 表示一项当前正在执行的额度及其实时用量。
type Usage struct {
	PolicyID   string
	PolicyName string
	Period     Period
	Used       int64
	Limit      int64
	Start      time.Time
	End        time.Time
}

// PolicySource 提供指定调用方当前命中的已启用策略。
type PolicySource interface {
	ActivePolicies(callerID string) ([]Policy, error)
}

// Counter 保存当前自然周期内的实时 Token 使用量。
type Counter interface {
	Read(ctx context.Context, buckets []Bucket) ([]int64, error)
	Add(ctx context.Context, buckets []Bucket, tokens int64) error
}

// Limiter 按请求开始时的策略快照检查并结算额度。
type Limiter struct {
	policies PolicySource
	counter  Counter
}

// Session 保存一次模型调用开始时命中的额度周期。
// 策略在请求处理中发生修改时，本次调用仍结算到开始时检查过的计数器。
type Session struct {
	buckets []Bucket
}

// Rejection 记录拒绝本次模型调用的额度结果。
type Rejection struct {
	Period  Period
	ResetAt time.Time
}

// NewLimiter 创建 Token 额度执行器。
func NewLimiter(policies PolicySource, counter Counter) *Limiter {
	return &Limiter{policies: policies, counter: counter}
}

// Begin 在调用模型前检查所有命中策略的当前额度。
func (l *Limiter) Begin(ctx context.Context, callerID string, now time.Time) (*Session, *Rejection, error) {
	buckets, err := l.currentBuckets(callerID, now)
	if err != nil {
		return nil, nil, err
	}
	if len(buckets) == 0 {
		return nil, nil, nil
	}
	used, err := l.readCounters(ctx, buckets)
	if err != nil {
		return nil, nil, err
	}
	for i, bucket := range buckets {
		if used[i] < bucket.Limit {
			continue
		}
		return nil, &Rejection{
			Period:  bucket.Period,
			ResetAt: bucket.End,
		}, nil
	}
	return &Session{buckets: buckets}, nil, nil
}

// CurrentUsage 返回调用方当前实际命中的额度及其实时计数。
func (l *Limiter) CurrentUsage(ctx context.Context, callerID string, now time.Time) ([]Usage, error) {
	buckets, err := l.currentBuckets(callerID, now)
	if err != nil {
		return nil, err
	}
	if len(buckets) == 0 {
		return []Usage{}, nil
	}
	used, err := l.readCounters(ctx, buckets)
	if err != nil {
		return nil, err
	}
	usages := make([]Usage, len(buckets))
	for i, bucket := range buckets {
		usages[i] = Usage{
			PolicyID:   bucket.PolicyID,
			PolicyName: bucket.PolicyName,
			Period:     bucket.Period,
			Used:       used[i],
			Limit:      bucket.Limit,
			Start:      bucket.Start,
			End:        bucket.End,
		}
	}
	return usages, nil
}

// Charge 使用模型厂商返回的实际 Token 结算本次调用。
// 当前调用可能使计数越过上限，下一次调用会在 Begin 阶段被拒绝。
func (l *Limiter) Charge(ctx context.Context, session *Session, tokens int64) error {
	if session == nil || tokens == 0 {
		return nil
	}
	if !tokenquotaconfig.IsValidTokenLimit(tokens) {
		return fmt.Errorf("charge token count %d is outside the supported range", tokens)
	}
	if err := l.counter.Add(ctx, session.buckets, tokens); err != nil {
		return fmt.Errorf("add token quota counters: %w", err)
	}
	return nil
}

func (l *Limiter) readCounters(ctx context.Context, buckets []Bucket) ([]int64, error) {
	used, err := l.counter.Read(ctx, buckets)
	if err != nil {
		return nil, fmt.Errorf("read token quota counters: %w", err)
	}
	if len(used) != len(buckets) {
		return nil, fmt.Errorf(
			"read %d token quota counters for %d buckets",
			len(used),
			len(buckets),
		)
	}
	for i, value := range used {
		if value < 0 {
			return nil, fmt.Errorf("token quota counter %d is negative", i)
		}
	}
	return used, nil
}

func (l *Limiter) currentBuckets(callerID string, now time.Time) ([]Bucket, error) {
	if !resourceconfig.IsCanonicalID(callerID) {
		return nil, errors.New("caller ID must be a canonical UUID")
	}
	policies, err := l.policies.ActivePolicies(callerID)
	if err != nil {
		return nil, err
	}
	if len(policies) > tokenquotaconfig.MaxPoliciesPerCaller {
		return nil, fmt.Errorf(
			"caller %q matches %d token quota policies; limit is %d",
			callerID,
			len(policies),
			tokenquotaconfig.MaxPoliciesPerCaller,
		)
	}
	policies = slices.Clone(policies)
	slices.SortFunc(policies, func(left, right Policy) int {
		return cmp.Compare(left.ID, right.ID)
	})

	buckets := make([]Bucket, 0, len(policies)*tokenquotaconfig.MaxLimits)
	seenPolicyIDs := make(map[string]bool, len(policies))
	for policyIndex, policy := range policies {
		if !resourceconfig.IsCanonicalID(policy.ID) {
			return nil, fmt.Errorf("token quota policy %d has an invalid ID", policyIndex)
		}
		if seenPolicyIDs[policy.ID] {
			return nil, fmt.Errorf("token quota policy %d duplicates ID %q", policyIndex, policy.ID)
		}
		seenPolicyIDs[policy.ID] = true
		if !resourceconfig.IsValidDisplayName(policy.Name) {
			return nil, fmt.Errorf("token quota policy %q has an invalid name", policy.ID)
		}
		if policy.TimeZone == nil {
			return nil, fmt.Errorf("token quota policy %q has no time zone", policy.ID)
		}
		if len(policy.Limits) == 0 || len(policy.Limits) > tokenquotaconfig.MaxLimits {
			return nil, fmt.Errorf("token quota policy %q has invalid limit count", policy.ID)
		}
		seenPeriods := make(map[Period]bool, len(policy.Limits))
		for limitIndex, limit := range policy.Limits {
			if seenPeriods[limit.Period] {
				return nil, fmt.Errorf(
					"token quota policy %q limit %d duplicates period %q",
					policy.ID,
					limitIndex,
					limit.Period,
				)
			}
			seenPeriods[limit.Period] = true
			if !tokenquotaconfig.IsValidTokenLimit(limit.Tokens) {
				return nil, fmt.Errorf(
					"token quota policy %q limit %d has invalid token count",
					policy.ID,
					limitIndex,
				)
			}
			start, end, err := periodWindow(now, policy.TimeZone, limit.Period)
			if err != nil {
				return nil, fmt.Errorf(
					"token quota policy %q limit %d: %w",
					policy.ID,
					limitIndex,
					err,
				)
			}
			buckets = append(buckets, Bucket{
				CallerID:   callerID,
				PolicyID:   policy.ID,
				PolicyName: policy.Name,
				Period:     limit.Period,
				Start:      start,
				End:        end,
				Limit:      limit.Tokens,
			})
		}
	}
	slices.SortFunc(buckets, func(left, right Bucket) int {
		if result := cmp.Compare(left.PolicyID, right.PolicyID); result != 0 {
			return result
		}
		return cmp.Compare(periodOrder(left.Period), periodOrder(right.Period))
	})
	return buckets, nil
}

func periodOrder(period Period) int {
	switch period {
	case PeriodDay:
		return 1
	case PeriodWeek:
		return 2
	case PeriodMonth:
		return 3
	default:
		return 4
	}
}

func periodWindow(
	now time.Time,
	location *time.Location,
	period Period,
) (time.Time, time.Time, error) {
	if location == nil {
		return time.Time{}, time.Time{}, errors.New("time zone is nil")
	}
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	switch period {
	case PeriodDay:
		return start, start.AddDate(0, 0, 1), nil
	case PeriodWeek:
		daysSinceMonday := (int(start.Weekday()) + 6) % 7
		start = start.AddDate(0, 0, -daysSinceMonday)
		return start, start.AddDate(0, 0, 7), nil
	case PeriodMonth:
		start = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
		return start, start.AddDate(0, 1, 0), nil
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unsupported period %q", period)
	}
}

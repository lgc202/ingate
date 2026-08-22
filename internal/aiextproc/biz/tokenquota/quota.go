// Package tokenquota 执行调用方模型 Token 额度检查和结算
package tokenquota

import (
	"context"
	"fmt"
	"time"
)

// Period 表示额度对应的自然周期
type Period string

const (
	PeriodDay   Period = "day"
	PeriodWeek  Period = "week"
	PeriodMonth Period = "month"
)

// Limit 定义一个自然周期内允许使用的总 Token 数
type Limit struct {
	Period Period
	Tokens int64
}

// Policy 是请求执行阶段需要的最小 Token 额度配置
type Policy struct {
	ID       string
	Name     string
	TimeZone *time.Location
	Limits   []Limit
}

// Bucket 标识一个调用方在某项策略和自然周期内的计数器
type Bucket struct {
	CallerID   string
	PolicyID   string
	PolicyName string
	Period     Period
	Start      time.Time
	End        time.Time
	Limit      int64
}

// Usage 表示一项当前正在执行的额度及其实时用量
type Usage struct {
	PolicyID   string
	PolicyName string
	Period     Period
	Used       int64
	Limit      int64
	Start      time.Time
	End        time.Time
}

// PolicySource 提供指定调用方当前命中的已启用策略
type PolicySource interface {
	ActivePolicies(callerID string) ([]Policy, error)
}

// Counter 保存当前自然周期内的实时 Token 使用量
type Counter interface {
	Read(ctx context.Context, buckets []Bucket) ([]int64, error)
	Add(ctx context.Context, buckets []Bucket, tokens int64) error
}

// Service 按请求开始时的策略快照检查并结算额度
type Service struct {
	policies PolicySource
	counter  Counter
}

// Session 保存一次模型调用开始时命中的额度周期
// 策略在请求处理中发生修改时，本次调用仍结算到开始时检查过的计数器
type Session struct {
	buckets []Bucket
}

// Exceeded 描述阻止本次调用的额度
type Exceeded struct {
	Period  Period
	ResetAt time.Time
}

// NewService 创建 Token 额度执行服务
func NewService(policies PolicySource, counter Counter) *Service {
	return &Service{policies: policies, counter: counter}
}

// Begin 在调用模型前检查所有命中策略的当前额度
func (s *Service) Begin(ctx context.Context, callerID string, now time.Time) (*Session, *Exceeded, error) {
	buckets, err := s.currentBuckets(callerID, now)
	if err != nil {
		return nil, nil, err
	}
	if len(buckets) == 0 {
		return nil, nil, nil
	}
	used, err := s.counter.Read(ctx, buckets)
	if err != nil {
		return nil, nil, fmt.Errorf("read token quota counters: %w", err)
	}
	for i, bucket := range buckets {
		if used[i] < bucket.Limit {
			continue
		}
		return nil, &Exceeded{
			Period:  bucket.Period,
			ResetAt: bucket.End,
		}, nil
	}
	return &Session{buckets: buckets}, nil, nil
}

// CurrentUsage 返回调用方当前实际命中的额度及其实时计数
func (s *Service) CurrentUsage(ctx context.Context, callerID string, now time.Time) ([]Usage, error) {
	buckets, err := s.currentBuckets(callerID, now)
	if err != nil {
		return nil, err
	}
	if len(buckets) == 0 {
		return []Usage{}, nil
	}
	used, err := s.counter.Read(ctx, buckets)
	if err != nil {
		return nil, fmt.Errorf("read token quota counters: %w", err)
	}
	usages := make([]Usage, 0, len(buckets))
	for i, bucket := range buckets {
		usages = append(usages, Usage{
			PolicyID:   bucket.PolicyID,
			PolicyName: bucket.PolicyName,
			Period:     bucket.Period,
			Used:       used[i],
			Limit:      bucket.Limit,
			Start:      bucket.Start,
			End:        bucket.End,
		})
	}
	return usages, nil
}

// Charge 使用模型厂商返回的实际 Token 结算本次调用
// 当前调用可能使计数越过上限，下一次调用会在 Begin 阶段被拒绝
func (s *Service) Charge(ctx context.Context, session *Session, tokens int64) error {
	if session == nil || tokens <= 0 {
		return nil
	}
	if err := s.counter.Add(ctx, session.buckets, tokens); err != nil {
		return fmt.Errorf("add token quota counters: %w", err)
	}
	return nil
}

func (s *Service) currentBuckets(callerID string, now time.Time) ([]Bucket, error) {
	policies, err := s.policies.ActivePolicies(callerID)
	if err != nil {
		return nil, err
	}
	buckets := make([]Bucket, 0, len(policies)*3)
	for _, policy := range policies {
		for _, limit := range policy.Limits {
			start, end := periodWindow(now, policy.TimeZone, limit.Period)
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
	return buckets, nil
}

func periodWindow(now time.Time, location *time.Location, period Period) (time.Time, time.Time) {
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	switch period {
	case PeriodDay:
		return start, start.AddDate(0, 0, 1)
	case PeriodWeek:
		daysSinceMonday := (int(start.Weekday()) + 6) % 7
		start = start.AddDate(0, 0, -daysSinceMonday)
		return start, start.AddDate(0, 0, 7)
	case PeriodMonth:
		start = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
		return start, start.AddDate(0, 1, 0)
	default:
		panic("unsupported token quota period")
	}
}

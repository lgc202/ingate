// Package ratelimit 实现请求进入上游前的固定窗口限流规则
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Subject 表示计数器的划分方式
type Subject string

const (
	// SubjectShared 表示同一策略作用域的请求共享计数器
	SubjectShared Subject = "Shared"
	// SubjectIP 表示每个客户端 IP 使用独立计数器
	SubjectIP Subject = "IP"
	// SubjectHeader 表示每个指定 Header 值使用独立计数器
	SubjectHeader Subject = "Header"
)

var ErrInvalidRule = errors.New("invalid rate limit rule")

// Rule 是 Controller 已经完成目标解析后的可执行限流规则
type Rule struct {
	PolicyID      string
	Scope         string
	Subject       Subject
	HeaderName    string
	Requests      int64
	WindowSeconds int64
}

// Request 保存计算限流主体需要的最小请求信息
type Request struct {
	ClientIP string
	Headers  map[string]string
}

// Bucket 表示 Redis 中一个固定时间窗口计数器
type Bucket struct {
	PolicyID string
	Scope    string
	Subject  string
	Limit    int64
	Start    time.Time
	End      time.Time
}

// Decision 是一次原子计数后的准入结果
type Decision struct {
	Allowed bool
}

// Counter 在共享存储中原子检查并递增一个窗口计数器
type Counter interface {
	Acquire(context.Context, Bucket) (Decision, error)
}

// Exceeded 表示请求命中的具体限流规则已经耗尽
type Exceeded struct {
	PolicyID   string
	RetryAfter time.Duration
}

// Service 按稳定顺序执行当前 Route 命中的全部限流规则
type Service struct {
	counter Counter
}

// NewService 创建请求限流服务
func NewService(counter Counter) *Service {
	return &Service{counter: counter}
}

// Admit 检查并占用一次请求额度
func (s *Service) Admit(ctx context.Context, rules []Rule, request Request, now time.Time) (*Exceeded, error) {
	ordered := slices.Clone(rules)
	slices.SortFunc(ordered, func(left, right Rule) int {
		if result := strings.Compare(left.PolicyID, right.PolicyID); result != 0 {
			return result
		}
		return strings.Compare(left.Scope, right.Scope)
	})
	// 每条策略使用独立 Redis Key 和独立 Lua 调用，既保持策略统计独立，也避免 Redis Cluster
	// 中跨 hash slot 的多 Key 脚本限制；请求被后续策略拒绝时，已通过的前序策略仍计入该次尝试
	for _, rule := range ordered {
		bucket, err := newBucket(rule, request, now)
		if err != nil {
			return nil, err
		}
		decision, err := s.counter.Acquire(ctx, bucket)
		if err != nil {
			return nil, fmt.Errorf("acquire request rate limit: %w", err)
		}
		if !decision.Allowed {
			retryAfter := bucket.End.Sub(now)
			if retryAfter < time.Second {
				retryAfter = time.Second
			}
			return &Exceeded{PolicyID: rule.PolicyID, RetryAfter: retryAfter}, nil
		}
	}
	return nil, nil
}

func newBucket(rule Rule, request Request, now time.Time) (Bucket, error) {
	if rule.PolicyID == "" || rule.Scope == "" || rule.Requests < 1 || rule.WindowSeconds < 1 {
		return Bucket{}, ErrInvalidRule
	}
	var subject string
	switch rule.Subject {
	case SubjectShared:
		subject = "shared"
	case SubjectIP:
		subject = strings.TrimSpace(request.ClientIP)
		if subject == "" {
			return Bucket{}, fmt.Errorf("%w: client IP is missing", ErrInvalidRule)
		}
	case SubjectHeader:
		if rule.HeaderName == "" {
			return Bucket{}, fmt.Errorf("%w: subject header is missing", ErrInvalidRule)
		}
		subject = request.Headers[strings.ToLower(rule.HeaderName)]
	default:
		return Bucket{}, fmt.Errorf("%w: unsupported subject %q", ErrInvalidRule, rule.Subject)
	}

	window := time.Duration(rule.WindowSeconds) * time.Second
	start := time.Unix(now.Unix()-now.Unix()%rule.WindowSeconds, 0).UTC()
	return Bucket{
		PolicyID: rule.PolicyID,
		Scope:    rule.Scope,
		Subject:  subject,
		Limit:    rule.Requests,
		Start:    start,
		End:      start.Add(window),
	}, nil
}

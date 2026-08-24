// Package ratelimit 实现请求进入上游前的共享限流规则
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"math"
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

// Limit 表示解析计数对象后的一条可执行速率限制
type Limit struct {
	PolicyID string
	Scope    string
	Subject  string
	Requests int
	Period   time.Duration
}

// Decision 是共享计数器返回的准入结果
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

// Counter 在共享存储中原子消费一次请求额度
type Counter interface {
	Allow(context.Context, Limit) (Decision, error)
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
func (s *Service) Admit(ctx context.Context, rules []Rule, request Request) (*Exceeded, error) {
	ordered := slices.Clone(rules)
	slices.SortFunc(ordered, func(left, right Rule) int {
		if result := strings.Compare(left.PolicyID, right.PolicyID); result != 0 {
			return result
		}
		return strings.Compare(left.Scope, right.Scope)
	})
	// 每条策略使用独立 Redis Key，既保持策略统计独立，也避免 Redis Cluster 跨 slot 操作
	// 请求被后续策略拒绝时，已通过的前序策略仍计入该次尝试
	for _, rule := range ordered {
		limit, err := rule.limit(request)
		if err != nil {
			return nil, err
		}
		decision, err := s.counter.Allow(ctx, limit)
		if err != nil {
			return nil, fmt.Errorf("apply request rate limit: %w", err)
		}
		if !decision.Allowed {
			return &Exceeded{PolicyID: rule.PolicyID, RetryAfter: decision.RetryAfter}, nil
		}
	}
	return nil, nil
}

func (r Rule) limit(request Request) (Limit, error) {
	if r.PolicyID == "" || r.Scope == "" || r.Requests < 1 || r.Requests > math.MaxInt || r.WindowSeconds < 1 {
		return Limit{}, ErrInvalidRule
	}
	var subject string
	switch r.Subject {
	case SubjectShared:
		subject = "shared"
	case SubjectIP:
		subject = strings.TrimSpace(request.ClientIP)
		if subject == "" {
			return Limit{}, fmt.Errorf("%w: client IP is missing", ErrInvalidRule)
		}
	case SubjectHeader:
		if r.HeaderName == "" {
			return Limit{}, fmt.Errorf("%w: subject header is missing", ErrInvalidRule)
		}
		subject = request.Headers[strings.ToLower(r.HeaderName)]
	default:
		return Limit{}, fmt.Errorf("%w: unsupported subject %q", ErrInvalidRule, r.Subject)
	}

	return Limit{
		PolicyID: r.PolicyID,
		Scope:    r.Scope,
		Subject:  subject,
		Requests: int(r.Requests),
		Period:   time.Duration(r.WindowSeconds) * time.Second,
	}, nil
}

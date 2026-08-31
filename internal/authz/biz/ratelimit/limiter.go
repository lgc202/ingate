// Package ratelimit 实现请求进入上游前的共享限流规则。
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/http/httpguts"

	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

const (
	// SubjectShared 表示同一策略作用域的请求共享计数器。
	SubjectShared Subject = "Shared"
	// SubjectIP 表示每个客户端 IP 使用独立计数器。
	SubjectIP Subject = "IP"
	// SubjectHeader 表示每个指定 Header 值使用独立计数器。
	SubjectHeader Subject = "Header"

	maxWindowSeconds = math.MaxInt64 / int64(time.Second)
)

// ErrInvalidRule 表示 Controller 下发了 Authz 无法执行的限流规则。
var ErrInvalidRule = errors.New("invalid rate limit rule")

// Subject 表示计数器的划分方式。
type Subject string

// Rule 是 Controller 已经完成目标解析后的可执行限流规则。
type Rule struct {
	PolicyID      string
	Scope         string
	Subject       Subject
	HeaderName    string
	Requests      int64
	WindowSeconds int64
}

// Request 保存计算限流主体需要的最小请求信息。
type Request struct {
	ClientIP string
	Headers  map[string]string
}

// Limit 表示解析计数对象后的一条可执行速率限制。
type Limit struct {
	PolicyID string
	Scope    string
	Subject  string
	Requests int
	Period   time.Duration
}

// Decision 是共享计数器返回的准入结果。
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

// Counter 在共享存储中原子消费一次请求额度。
type Counter interface {
	Allow(context.Context, Limit) (Decision, error)
}

// Rejection 记录拒绝当前请求的限流结果。
type Rejection struct {
	RetryAfter time.Duration
}

// Limiter 按稳定顺序执行当前 Route 命中的全部限流规则。
type Limiter struct {
	counter Counter
}

// NewLimiter 创建请求限流器。
func NewLimiter(counter Counter) *Limiter {
	return &Limiter{counter: counter}
}

// Admit 检查并占用一次请求额度。
func (l *Limiter) Admit(ctx context.Context, rules []Rule, request Request) (*Rejection, error) {
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
		decision, err := l.counter.Allow(ctx, limit)
		if err != nil {
			return nil, fmt.Errorf("apply request rate limit: %w", err)
		}
		if !decision.Allowed {
			return &Rejection{RetryAfter: decision.RetryAfter}, nil
		}
	}
	return nil, nil
}

func (r Rule) limit(request Request) (Limit, error) {
	switch {
	case !resourceconfig.IsCanonicalID(r.PolicyID):
		return Limit{}, fmt.Errorf("%w: policy ID is invalid", ErrInvalidRule)
	case r.Scope == "":
		return Limit{}, fmt.Errorf("%w: scope is missing", ErrInvalidRule)
	case r.Requests < 1 || r.Requests > math.MaxInt:
		return Limit{}, fmt.Errorf("%w: request limit %d is out of range", ErrInvalidRule, r.Requests)
	case r.WindowSeconds < 1 || r.WindowSeconds > maxWindowSeconds:
		return Limit{}, fmt.Errorf("%w: window %d is out of range", ErrInvalidRule, r.WindowSeconds)
	}

	var subject string
	switch r.Subject {
	case SubjectShared:
		if r.HeaderName != "" {
			return Limit{}, fmt.Errorf("%w: shared subject contains a header name", ErrInvalidRule)
		}
		subject = "shared"
	case SubjectIP:
		if r.HeaderName != "" {
			return Limit{}, fmt.Errorf("%w: IP subject contains a header name", ErrInvalidRule)
		}
		subject = strings.TrimSpace(request.ClientIP)
		if subject == "" {
			return Limit{}, fmt.Errorf("%w: client IP is missing", ErrInvalidRule)
		}
	case SubjectHeader:
		if !httpguts.ValidHeaderFieldName(r.HeaderName) || strings.ToLower(r.HeaderName) != r.HeaderName {
			return Limit{}, fmt.Errorf("%w: subject header %q is invalid", ErrInvalidRule, r.HeaderName)
		}
		subject = request.Headers[r.HeaderName]
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

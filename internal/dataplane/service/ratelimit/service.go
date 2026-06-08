// Package ratelimit 实现数据面限流服务
package ratelimit

import (
	"context"
	"errors"
	"log/slog"
	"time"

	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
	"github.com/lgc202/ingate/pkg/xredis"
)

const defaultCommandTimeout = 50 * time.Millisecond

// Service 承载限流检查用例
type Service struct {
	logger  *slog.Logger
	clients *xredis.Manager
}

// NewService 创建限流服务
func NewService(logger *slog.Logger) *Service {
	return &Service{
		logger:  logger,
		clients: xredis.NewManager(),
	}
}

// Check 执行一组 Redis-backed 限流检查
func (s *Service) Check(ctx context.Context, request dataplaneratelimit.CheckRequest) dataplaneratelimit.CheckResponse {
	if len(request.Checks) == 0 {
		return dataplaneratelimit.CheckResponse{}
	}

	results := make([]dataplaneratelimit.Result, 0, len(request.Checks))
	for _, check := range request.Checks {
		result, err := s.executeCheck(ctx, check)
		if err != nil {
			s.logger.Error("rate limit check failed", "policy", check.PolicyName, "rule", check.RuleName, "redis_store", check.RedisStore.ID, "err", err)
			result = dataplaneratelimit.Result{
				PolicyName: check.PolicyName,
				RuleName:   check.RuleName,
				Allowed:    false,
				Error:      err.Error(),
			}
		}
		results = append(results, result)
	}

	return dataplaneratelimit.CheckResponse{
		Results: results,
	}
}

func (s *Service) executeCheck(ctx context.Context, check dataplaneratelimit.Check) (dataplaneratelimit.Result, error) {
	if check.RedisKey == "" {
		return dataplaneratelimit.Result{}, errors.New("redis key is required")
	}
	if check.Limit.Requests <= 0 || check.Limit.WindowSeconds <= 0 {
		return dataplaneratelimit.Result{}, errors.New("limit requests and windowSeconds must be greater than zero")
	}

	timeout := defaultCommandTimeout
	if check.TimeoutMillis > 0 {
		timeout = time.Duration(check.TimeoutMillis) * time.Millisecond
	} else if check.RedisStore.CommandTimeoutMillis > 0 {
		timeout = time.Duration(check.RedisStore.CommandTimeoutMillis) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := s.clients.Client(redisConfig(check.RedisStore))
	if err != nil {
		return dataplaneratelimit.Result{}, err
	}
	switch check.Algorithm {
	case "", dataplaneratelimit.AlgorithmFixedWindow:
		return fixedWindow(ctx, client, check)
	case dataplaneratelimit.AlgorithmSlidingWindow:
		return slidingWindow(ctx, client, check)
	case dataplaneratelimit.AlgorithmTokenBucket:
		return tokenBucket(ctx, client, check)
	default:
		return dataplaneratelimit.Result{}, errors.New("unsupported rate limit algorithm")
	}
}

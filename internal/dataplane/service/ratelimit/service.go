// Package ratelimit 实现数据面限流服务
package ratelimit

import (
	"context"
	"log/slog"

	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
	"github.com/lgc202/ingate/pkg/xredis"
)

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

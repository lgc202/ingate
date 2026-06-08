// Package service 组织数据面服务的一等能力
package service

import (
	"log/slog"

	"github.com/lgc202/ingate/internal/dataplane/service/ratelimit"
)

// Service 聚合数据面服务对外提供的能力
type Service struct {
	RateLimit *ratelimit.Service
}

// New 创建数据面服务集合
func New(logger *slog.Logger) *Service {
	return &Service{
		RateLimit: ratelimit.NewService(logger.With("capability", "rate-limit")),
	}
}

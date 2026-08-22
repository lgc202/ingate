// Package aiusage 提供控制台模型调用与 Token 用量分析
package aiusage

import (
	"context"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

const (
	defaultBreakdownLimit       = 10
	reasonDependencyUnavailable = "DEPENDENCY_UNAVAILABLE"
)

// ErrUnavailable 表示模型用量分析当前无法提供查询
var ErrUnavailable = kratoserrors.ServiceUnavailable(reasonDependencyUnavailable, "AI usage analytics unavailable").
	WithMetadata(map[string]string{"user_message": "AI 用量分析服务暂时不可用，请稍后重试"})

// Unavailable 保留 Analytics 返回的底层原因，同时向控制台暴露稳定错误语义
func Unavailable(cause error) error {
	return ErrUnavailable.WithCause(cause)
}

// Repository 定义 Admin API 查询 Analytics 所需的模型用量聚合能力
type Repository interface {
	Analyze(context.Context, Query) (Analysis, error)
}

// Service 提供不依赖 Analytics gRPC 协议的模型用量分析
type Service struct {
	repository Repository
}

// NewService 创建模型用量分析业务服务
func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// Analyze 查询同一范围内的模型用量汇总、趋势与排名
func (s *Service) Analyze(ctx context.Context, query Query) (Analysis, error) {
	if query.Limit == 0 {
		query.Limit = defaultBreakdownLimit
	}
	if query.Order == 0 {
		query.Order = BreakdownOrderCallCount
	}
	query.Bucket = bucketForRange(query.Filter.EndTime.Sub(query.Filter.StartTime))
	return s.repository.Analyze(ctx, query)
}

// bucketForRange 限制趋势点数量，短时间保留细节，长时间避免返回过密数据
func bucketForRange(value time.Duration) TimeBucket {
	switch {
	case value <= 2*time.Hour:
		return TimeBucketMinute
	case value <= 24*time.Hour:
		return TimeBucketFiveMinutes
	case value <= 7*24*time.Hour:
		return TimeBucketHour
	default:
		return TimeBucketDay
	}
}

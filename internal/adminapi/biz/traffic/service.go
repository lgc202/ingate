// Package traffic 提供控制台流量分析用例
package traffic

import (
	"context"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

// ErrUnavailable 表示流量分析组件当前无法提供查询
var ErrUnavailable = kratoserrors.ServiceUnavailable(adminv1.ErrorReason_DEPENDENCY_UNAVAILABLE.String(), "traffic analytics unavailable").
	WithMetadata(map[string]string{"user_message": "流量分析服务暂时不可用，请稍后重试"})

const defaultBreakdownLimit = 10

// Unavailable 保留 Analytics 返回的底层原因，同时向控制台暴露稳定错误语义
func Unavailable(cause error) error {
	return ErrUnavailable.WithCause(cause)
}

// Repository 定义 Admin API 查询 Analytics 所需的流量聚合能力
type Repository interface {
	Analyze(context.Context, Query) (Analysis, error)
	BatchGetResourceTraffic(context.Context, ResourceTrafficQuery) ([]ResourceTrafficSummary, error)
}

// Service 提供不依赖 Analytics gRPC 协议的流量分析
type Service struct {
	repository Repository
}

// NewService 创建流量分析业务服务
func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// Analyze 查询同一范围内的汇总、趋势和资源排名
func (s *Service) Analyze(ctx context.Context, query Query) (Analysis, error) {
	if query.Limit == 0 {
		query.Limit = defaultBreakdownLimit
	}
	if query.Order == 0 {
		query.Order = BreakdownOrderRequestCount
	}
	query.Bucket = bucketForRange(query.Filter.EndTime.Sub(query.Filter.StartTime))
	return s.repository.Analyze(ctx, query)
}

// BatchGetResourceTraffic 查询指定资源的列表流量摘要
func (s *Service) BatchGetResourceTraffic(ctx context.Context, query ResourceTrafficQuery) ([]ResourceTrafficSummary, error) {
	return s.repository.BatchGetResourceTraffic(ctx, query)
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

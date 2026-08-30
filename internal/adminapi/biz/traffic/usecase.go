// Package traffic 提供控制台流量分析用例。
package traffic

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

const defaultBreakdownLimit = 10

// ErrUnavailable 表示流量分析组件当前无法提供查询。
var ErrUnavailable = errors.ServiceUnavailable(
	adminv1.ErrorReason_DEPENDENCY_UNAVAILABLE.String(),
	"流量分析服务暂时不可用，请稍后重试",
)

// Analyzer 定义流量分析所需的聚合查询能力。
type Analyzer interface {
	Analyze(ctx context.Context, query Query) (Analysis, error)
	BatchGetResourceTraffic(
		ctx context.Context,
		query ResourceTrafficQuery,
	) ([]ResourceTrafficSummary, error)
}

// Usecase 提供不依赖 Analytics gRPC 协议的流量分析。
type Usecase struct {
	analyzer Analyzer
}

// NewUsecase 创建流量分析用例。
func NewUsecase(analyzer Analyzer) *Usecase {
	return &Usecase{analyzer: analyzer}
}

// Analyze 查询同一范围内的汇总、趋势和资源排名。
func (uc *Usecase) Analyze(ctx context.Context, query Query) (Analysis, error) {
	if query.Limit == 0 {
		query.Limit = defaultBreakdownLimit
	}
	if query.Order == 0 {
		query.Order = BreakdownOrderRequestCount
	}
	query.Bucket = bucketForRange(query.Filter.EndTime.Sub(query.Filter.StartTime))
	return uc.analyzer.Analyze(ctx, query)
}

// BatchGetResourceTraffic 查询指定资源的列表流量摘要。
func (uc *Usecase) BatchGetResourceTraffic(
	ctx context.Context,
	query ResourceTrafficQuery,
) ([]ResourceTrafficSummary, error) {
	return uc.analyzer.BatchGetResourceTraffic(ctx, query)
}

// Unavailable 保留 Analytics 返回的底层原因，同时向控制台暴露稳定错误语义。
func Unavailable(cause error) error {
	return ErrUnavailable.WithCause(cause)
}

func bucketForRange(duration time.Duration) TimeBucket {
	// 短时间范围保留细节，长时间范围限制趋势点数量。
	switch {
	case duration <= 2*time.Hour:
		return TimeBucketMinute
	case duration <= 24*time.Hour:
		return TimeBucketFiveMinutes
	case duration <= 7*24*time.Hour:
		return TimeBucketHour
	default:
		return TimeBucketDay
	}
}

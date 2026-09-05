// Package aiusage 提供控制台模型调用与 Token 用量分析。
package aiusage

import (
	"cmp"
	"context"

	"github.com/lgc202/ingate/internal/adminapi/biz/analysisquery"
	"github.com/lgc202/ingate/internal/adminapi/biz/apperror"
)

const defaultBreakdownLimit = 10

// ErrUnavailable 表示模型用量分析当前无法提供查询。
var ErrUnavailable = apperror.DependencyUnavailable("AI 用量分析服务暂时不可用，请稍后重试", nil)

// Analyzer 定义模型用量分析所需的聚合查询能力。
type Analyzer interface {
	Analyze(context.Context, Query) (Analysis, error)
}

// Usecase 提供不依赖 Analytics gRPC 协议的模型用量分析。
type Usecase struct {
	analyzer Analyzer
}

// NewUsecase 创建模型用量分析用例。
func NewUsecase(analyzer Analyzer) *Usecase {
	return &Usecase{analyzer: analyzer}
}

// Analyze 查询同一范围内的模型用量汇总、趋势与排名。
func (uc *Usecase) Analyze(ctx context.Context, query Query) (Analysis, error) {
	query.Limit = cmp.Or(query.Limit, defaultBreakdownLimit)
	query.Order = cmp.Or(query.Order, BreakdownOrderCallCount)
	query.Bucket = TimeBucket(analysisquery.TimeBucketForRange(query.Filter.EndTime.Sub(query.Filter.StartTime)))
	return uc.analyzer.Analyze(ctx, query)
}

// Unavailable 保留 Analytics 返回的底层原因，同时向控制台暴露稳定错误语义。
func Unavailable(cause error) error {
	return apperror.DependencyUnavailable("AI 用量分析服务暂时不可用，请稍后重试", cause)
}

package tokenquota

import (
	"context"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

const reasonUsageUnavailable = "DEPENDENCY_UNAVAILABLE"

// Usage 表示调用方当前命中的一项 Token 额度及其实时计数
type Usage struct {
	PolicyID   string
	PolicyName string
	Period     Period
	Used       int64
	Limit      int64
	Start      time.Time
	ResetAt    time.Time
}

// Period 表示 Token 额度的自然周期
type Period string

const (
	PeriodDay   Period = "day"
	PeriodWeek  Period = "week"
	PeriodMonth Period = "month"
)

// UsageReader 定义 Admin API 查询 AI ExtProc 实时额度所需的能力
type UsageReader interface {
	Current(ctx context.Context, callerID string) ([]Usage, error)
}

// UsageUnavailable 保留内部调用失败原因，并向控制台返回稳定错误语义
func UsageUnavailable(cause error) error {
	return kratoserrors.ServiceUnavailable(reasonUsageUnavailable, "token quota usage unavailable").
		WithMetadata(map[string]string{"user_message": "实时额度暂时不可用，请稍后重试"}).
		WithCause(cause)
}

package tokenquota

import "time"

const (
	// PeriodDay 表示调用方配置时区中的自然日。
	PeriodDay Period = "day"
	// PeriodWeek 表示调用方配置时区中从周一开始的自然周。
	PeriodWeek Period = "week"
	// PeriodMonth 表示调用方配置时区中的自然月。
	PeriodMonth Period = "month"
)

// Period 表示 Token 额度的自然周期。
type Period string

// Usage 表示调用方当前命中的一项 Token 额度及其实时计数。
type Usage struct {
	PolicyID   string
	PolicyName string
	Period     Period
	Used       int64
	Limit      int64
	StartedAt  time.Time
	ResetAt    time.Time
}

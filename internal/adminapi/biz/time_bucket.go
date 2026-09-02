package biz

import "time"

// TimeBucket 是 Admin API 分析趋势使用的时间粒度。
type TimeBucket uint8

const (
	// TimeBucketMinute 表示每分钟一个趋势点。
	TimeBucketMinute TimeBucket = iota + 1
	// TimeBucketFiveMinutes 表示每五分钟一个趋势点。
	TimeBucketFiveMinutes
	// TimeBucketHour 表示每小时一个趋势点。
	TimeBucketHour
	// TimeBucketDay 表示每天一个趋势点。
	TimeBucketDay
)

// TimeBucketForRange 根据查询范围选择趋势时间粒度。
func TimeBucketForRange(duration time.Duration) TimeBucket {
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

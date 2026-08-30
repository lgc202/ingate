// Package analyticsconfig 定义请求分析链路共享的查询约束。
package analyticsconfig

import (
	"math"
	"time"
)

// MaxQueryRange 限制一次请求分析查询覆盖的时间跨度。
const MaxQueryRange = 90 * 24 * time.Hour

var (
	earliestTime = time.Unix(0, math.MinInt64).UTC()
	latestTime   = time.Unix(0, math.MaxInt64).UTC()
)

// IsSupportedTime 判断时间能否无损转换为 ClickHouse 查询使用的 Unix 纳秒。
func IsSupportedTime(value time.Time) bool {
	return !value.Before(earliestTime) && !value.After(latestTime)
}

// IsValidQueryRange 判断时间范围是否可表示、左闭右开且没有超过产品上限。
func IsValidQueryRange(startTime, endTime time.Time) bool {
	return IsSupportedTime(startTime) &&
		IsSupportedTime(endTime) &&
		startTime.Before(endTime) &&
		endTime.Sub(startTime) <= MaxQueryRange
}

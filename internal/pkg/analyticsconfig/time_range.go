// Package analyticsconfig 定义请求分析链路共享的查询约束。
package analyticsconfig

import (
	"math"
	"time"
)

// MaxQueryRange 限制一次请求分析查询覆盖的时间跨度。
const MaxQueryRange = 90 * 24 * time.Hour

var (
	// 明细表使用 DateTime64(9)，分钟聚合表使用范围更窄的 DateTime。
	// 共享边界取两种 ClickHouse 类型与 Unix 纳秒表示范围的交集；上界向下对齐到
	// 整分钟，确保范围末端仍能作为左闭右开查询的 end_time。
	earliestSupportedTime   = time.Unix(0, 0).UTC()
	supportedTimeUpperBound = time.Unix(math.MaxUint32, 0).UTC().Truncate(time.Minute)
)

// IsSupportedTime 判断时间是否位于分析链路可持久化和查询的共享范围内。
func IsSupportedTime(value time.Time) bool {
	return !value.Before(earliestSupportedTime) && value.Before(supportedTimeUpperBound)
}

// IsValidQueryRange 判断时间范围是否可表示、左闭右开且没有超过产品上限。
func IsValidQueryRange(startTime, endTime time.Time) bool {
	return IsSupportedTime(startTime) &&
		!endTime.After(supportedTimeUpperBound) &&
		startTime.Before(endTime) &&
		endTime.Sub(startTime) <= MaxQueryRange
}

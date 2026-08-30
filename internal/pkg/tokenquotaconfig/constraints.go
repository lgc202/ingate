// Package tokenquotaconfig 定义 Token 额度策略各信任边界共享的稳定领域约束。
package tokenquotaconfig

import (
	"strings"
	"time"

	// 嵌入 IANA 时区数据，避免最小容器缺少系统 zoneinfo 时拒绝合法策略。
	_ "time/tzdata"
)

const (
	// MaxLimits 是一条策略支持的自然周期额度数量。
	MaxLimits = 3
	// MaxPoliciesPerCaller 限制单个调用方可以同时命中的已启用策略数量。
	MaxPoliciesPerCaller = 64
	// MaxTokensPerPeriod 保证额度值可以被 JavaScript 无损表示。
	MaxTokensPerPeriod int64 = 1<<53 - 1
	// MaxTimeZoneBytes 限制 IANA 时区名称的存储大小。
	MaxTimeZoneBytes = 255
)

// LoadLocation 校验并加载规范化后的 IANA 时区。
func LoadLocation(value string) (string, *time.Location, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "Local" || len(value) > MaxTimeZoneBytes {
		return "", nil, false
	}
	location, err := time.LoadLocation(value)
	if err != nil {
		return "", nil, false
	}
	return value, location, true
}

// IsValidTokenLimit 判断 tokens 是否处于支持的额度范围内。
func IsValidTokenLimit(tokens int64) bool {
	return tokens > 0 && tokens <= MaxTokensPerPeriod
}

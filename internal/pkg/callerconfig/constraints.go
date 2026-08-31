// Package callerconfig 定义 Caller 各信任边界共享的稳定领域约束。
package callerconfig

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxRouteRefs 限制一个 Caller 可以直接授权的 Route 数量。
	MaxRouteRefs = 256
	// MaxAccessKeys 限制一个 Caller 可以保留的访问密钥数量，包括已停用密钥。
	MaxAccessKeys = 64
	// MaxAccessKeyDisplayNameBytes 限制访问密钥展示名称的存储大小。
	MaxAccessKeyDisplayNameBytes = 128
)

// IsValidAccessKeyDisplayName 判断 value 是否可以作为访问密钥展示名称持久化。
func IsValidAccessKeyDisplayName(value string) bool {
	if value == "" ||
		len(value) > MaxAccessKeyDisplayNameBytes ||
		!utf8.ValidString(value) ||
		strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

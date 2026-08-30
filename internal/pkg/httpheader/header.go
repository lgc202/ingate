// Package httpheader 提供控制面配置使用的 HTTP Header 规范化和校验。
package httpheader

import (
	"strings"

	"golang.org/x/net/http/httpguts"
)

const (
	// MaxNameBytes 限制单个 Header 名称的存储大小。
	MaxNameBytes = 256
	// MaxValueBytes 限制单个 Header 值的存储大小。
	MaxValueBytes = 8 << 10
)

// NormalizeName 规范化 Header 名称。
func NormalizeName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// NormalizeValue 规范化 Header 值。
func NormalizeValue(value string) string {
	return strings.TrimSpace(value)
}

// IsValidName 判断 value 是否为普通 HTTP Header 名称。
func IsValidName(value string) bool {
	return value != "" &&
		len(value) <= MaxNameBytes &&
		!strings.HasPrefix(value, ":") &&
		httpguts.ValidHeaderFieldName(value)
}

// IsValidValue 判断 value 是否可以安全写入 HTTP Header。
func IsValidValue(value string) bool {
	return len(value) <= MaxValueBytes && httpguts.ValidHeaderFieldValue(value)
}

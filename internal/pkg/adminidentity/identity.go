// Package adminidentity 定义管理面各进程共享的管理员身份协议。
package adminidentity

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// Header 是 Console 向内部管理服务传递已认证管理员标识的请求头。
	Header     = "X-Forwarded-User"
	maxIDBytes = 128
)

// IsValid 判断 value 是否可以作为跨服务传递和持久化隔离的管理员标识。
func IsValid(value string) bool {
	if value == "" || len(value) > maxIDBytes || !utf8.ValidString(value) ||
		strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

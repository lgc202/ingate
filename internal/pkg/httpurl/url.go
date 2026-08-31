// Package httpurl 定义各信任边界共享的 HTTP(S) URL 约束。
package httpurl

import (
	"net/url"
	"strings"
)

// MaxBytes 限制 HTTP(S) URL 的存储大小。
const MaxBytes = 4 << 10

// IsValid 判断 value 是否为可直接请求且不包含用户凭据的 HTTP(S) URL。
func IsValid(value string) bool {
	if value == "" ||
		len(value) > MaxBytes ||
		strings.TrimSpace(value) != value {
		return false
	}

	parsed, err := url.ParseRequestURI(value)
	return err == nil &&
		(parsed.Scheme == "http" || parsed.Scheme == "https") &&
		parsed.Host != "" &&
		parsed.User == nil &&
		parsed.Fragment == ""
}

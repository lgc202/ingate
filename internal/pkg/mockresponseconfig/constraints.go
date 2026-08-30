// Package mockresponseconfig 定义模拟响应策略各信任边界共享的稳定领域约束。
package mockresponseconfig

import (
	"mime"
	"strings"
)

const (
	// MinStatusCode 是模拟响应支持的最小 HTTP 状态码。
	MinStatusCode = 200
	// MaxStatusCode 是模拟响应支持的最大 HTTP 状态码。
	MaxStatusCode = 599
	// MaxHeaders 限制模拟响应可以附加的 Header 数量，不包含 Content-Type。
	MaxHeaders = 64
	// MaxContentTypeBytes 限制媒体类型配置的存储大小。
	MaxContentTypeBytes = 1 << 10
	// MaxBodyBytes 限制模拟响应正文的存储大小。
	MaxBodyBytes = 1 << 20
)

// IsValidStatusCode 判断 statusCode 是否处于模拟响应支持的范围内。
func IsValidStatusCode(statusCode int32) bool {
	return statusCode >= MinStatusCode && statusCode <= MaxStatusCode
}

// NormalizeContentType 校验并规范化 HTTP 媒体类型。
func NormalizeContentType(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > MaxContentTypeBytes {
		return "", false
	}
	mediaType, parameters, err := mime.ParseMediaType(value)
	if err != nil {
		return "", false
	}
	normalized := mime.FormatMediaType(mediaType, parameters)
	return normalized, normalized != ""
}

// IsReservedHeaderName 判断 Header 是否由 HTTP 栈或模拟响应配置的专用字段管理。
func IsReservedHeaderName(name string) bool {
	switch name {
	case "connection",
		"content-length",
		"content-type",
		"keep-alive",
		"proxy-connection",
		"trailer",
		"transfer-encoding",
		"upgrade":
		return true
	default:
		return false
	}
}

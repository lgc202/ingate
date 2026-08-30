// Package resourceconfig 定义声明式资源共享的产品字段约束。
package resourceconfig

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// MaxDisplayNameBytes 限制用户可编辑展示名称的存储大小。
const MaxDisplayNameBytes = 256

// IsValidDisplayName 判断 value 是否可以作为资源展示名称持久化。
func IsValidDisplayName(value string) bool {
	if value == "" ||
		len(value) > MaxDisplayNameBytes ||
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

// NormalizeID 校验并规范化声明式资源 ID。
func NormalizeID(value string) (string, bool) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", false
	}
	return id.String(), true
}

// IsCanonicalID 判断 value 是否为规范的小写 UUID。
func IsCanonicalID(value string) bool {
	id, valid := NormalizeID(value)
	return valid && id == value
}

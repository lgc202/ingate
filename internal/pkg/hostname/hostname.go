// Package hostname 提供 Gateway Host 的规范化和范围判断能力。
package hostname

import "strings"

// Normalize 将 Host 规范化为小写；空值和星号统一表示不限制 Host。
func Normalize(value string) (string, bool) {
	if value == "" || value == "*" {
		return "*", true
	}
	if strings.TrimSpace(value) != value {
		return "", false
	}

	value = strings.ToLower(value)
	if after, ok := strings.CutPrefix(value, "*."); ok {
		if !validDNSName(after) {
			return "", false
		}
		return value, true
	}
	return value, validDNSName(value)
}

// Overlaps 判断两个已规范化的 Host 范围是否会匹配同一请求域名。
func Overlaps(first, second string) bool {
	if first == "*" || second == "*" {
		return true
	}

	firstWildcard := strings.HasPrefix(first, "*.")
	secondWildcard := strings.HasPrefix(second, "*.")
	switch {
	case !firstWildcard && !secondWildcard:
		return first == second
	case firstWildcard && secondWildcard:
		firstSuffix := strings.TrimPrefix(first, "*")
		secondSuffix := strings.TrimPrefix(second, "*")
		return strings.HasSuffix(firstSuffix, secondSuffix) || strings.HasSuffix(secondSuffix, firstSuffix)
	case firstWildcard:
		return strings.HasSuffix(second, strings.TrimPrefix(first, "*"))
	default:
		return strings.HasSuffix(first, strings.TrimPrefix(second, "*"))
	}
}

func validDNSName(value string) bool {
	if value == "" || len(value) > 253 {
		return false
	}
	for label := range strings.SplitSeq(value, ".") {
		if label == "" || len(label) > 63 {
			return false
		}
		for i, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
			if (i == 0 || i == len(label)-1) && r == '-' {
				return false
			}
		}
	}
	return true
}

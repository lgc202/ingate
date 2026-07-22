package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"

	config "github.com/lgc202/ingate/pkg/plugin/tokenquota"
)

const (
	redisKeyPrefix       = "ingate-token-quota"
	missingSubjectValue  = "\x00"
	presentSubjectPrefix = "\x01"
	sharedSubjectValue   = "shared"
)

// RequiredHeaders 返回执行当前 RouteRule 的 Token 配额前需要读取的 Header
func RequiredHeaders(route config.RouteConfig) []string {
	seen := make(map[string]struct{})
	for _, policy := range route.Policies {
		if policy.Subject.Type == config.SubjectTypeHeader && policy.Subject.HeaderName != "" {
			seen[policy.Subject.HeaderName] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	slices.Sort(result)
	return result
}

// BuildChecks 将当前请求展开为需要检查的 Token 预算池
func BuildChecks(route config.RouteConfig, request RequestAttributes) []Check {
	checks := make([]Check, 0, len(route.Policies))
	for _, policy := range route.Policies {
		hash, ok := subjectHash(request, policy.Subject)
		if !ok {
			continue
		}
		checks = append(checks, Check{
			Policy:   policy,
			RedisKey: encodeKeySegments(redisKeyPrefix, policy.BudgetID, hash),
		})
	}
	return checks
}

func subjectHash(request RequestAttributes, subject config.Subject) (string, bool) {
	var value string
	switch subject.Type {
	case config.SubjectTypeShared:
		value = sharedSubjectValue
	case config.SubjectTypeIP:
		value = optionalSubjectValue(clientIP(request.RemoteAddr))
	case config.SubjectTypeHeader:
		value = optionalSubjectValue(headerValue(request.Headers, subject.HeaderName))
	default:
		return "", false
	}

	var key strings.Builder
	writeKeySegments(&key, string(subject.Type), subject.HeaderName, value)
	digest := sha256.Sum256([]byte(key.String()))
	return hex.EncodeToString(digest[:]), true
}

func optionalSubjectValue(value string) string {
	if value == "" {
		return missingSubjectValue
	}
	return presentSubjectPrefix + value
}

func headerValue(headers map[string]string, name string) string {
	if name == "" {
		return ""
	}
	if value := headers[name]; value != "" {
		return value
	}
	lowerName := strings.ToLower(name)
	for key, value := range headers {
		if strings.ToLower(key) == lowerName {
			return value
		}
	}
	return ""
}

func clientIP(remoteAddr string) string {
	if strings.HasPrefix(remoteAddr, "[") {
		if i := strings.Index(remoteAddr, "]"); i > 0 {
			return remoteAddr[1:i]
		}
	}
	if host, _, ok := strings.Cut(remoteAddr, ":"); ok && strings.Count(host, ".") == 3 {
		return host
	}
	if value, _, ok := strings.Cut(remoteAddr, ","); ok {
		return strings.TrimSpace(value)
	}
	return remoteAddr
}

func encodeKeySegments(segments ...string) string {
	var key strings.Builder
	writeKeySegments(&key, segments...)
	return key.String()
}

func writeKeySegments(key *strings.Builder, segments ...string) {
	for _, segment := range segments {
		key.WriteString(strconv.Itoa(len(segment)))
		key.WriteByte(':')
		key.WriteString(segment)
	}
}

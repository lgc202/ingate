package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"

	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

const (
	cookieHeader          = "cookie"
	missingKeyValue       = "\x00"
	presentKeyValuePrefix = "\x01"
)

// Request 表示限流判断需要读取的请求信息
type Request struct {
	GatewayName string
	RouteName   string
	RuleName    string
	Path        string
	RemoteAddr  string
	Headers     map[string]string
}

// HeaderNames 返回执行路由限流规则前需要从请求中读取的 header
func HeaderNames(route config.RouteConfig) []string {
	seen := make(map[string]struct{})
	for _, policy := range route.Policies {
		for _, rule := range policy.Rules {
			for _, part := range rule.Key {
				switch {
				case part.Type == config.KeyTypeHeader && part.Name != "":
					seen[part.Name] = struct{}{}
				case part.Type == config.KeyTypeCookie:
					seen[cookieHeader] = struct{}{}
				}
			}
		}
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	return result
}

func keyValue(req Request, part config.KeyPart) (string, bool) {
	switch part.Type {
	case config.KeyTypeIP:
		return optionalKeyValue(clientIP(req.RemoteAddr)), true
	case config.KeyTypeHeader:
		return optionalKeyValue(headerValue(req.Headers, part.Name)), true
	case config.KeyTypeQuery:
		values, err := url.ParseQuery(rawQuery(req.Path))
		if err != nil {
			return missingKeyValue, true
		}
		return optionalKeyValue(values.Get(part.Name)), true
	case config.KeyTypeCookie:
		return optionalKeyValue(cookieValue(headerValue(req.Headers, cookieHeader), part.Name)), true
	case config.KeyTypeGateway:
		return optionalKeyValue(req.GatewayName), true
	case config.KeyTypeRoute:
		return optionalKeyValue(req.RouteName), true
	case config.KeyTypeRouteRule:
		return optionalKeyValue(req.RuleName), true
	default:
		return "", false
	}
}

func optionalKeyValue(value string) string {
	if value == "" {
		return missingKeyValue
	}
	return presentKeyValuePrefix + value
}

func compositeKeyHash(req Request, parts []config.KeyPart) (string, bool) {
	if len(parts) == 0 {
		return "", false
	}
	var composite strings.Builder
	for _, part := range parts {
		value, ok := keyValue(req, part)
		if !ok {
			return "", false
		}
		writeKeySegments(&composite, string(part.Type), part.Name, value)
	}
	digest := sha256.Sum256([]byte(composite.String()))
	return hex.EncodeToString(digest[:]), true
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

func rawQuery(path string) string {
	if _, query, ok := strings.Cut(path, "?"); ok {
		return query
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

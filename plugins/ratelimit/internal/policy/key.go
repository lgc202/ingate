package policy

import (
	"net/url"
	"strings"

	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

const (
	apiKeyHeader         = "x-ingate-api-key"
	consumerHeader       = "x-ingate-consumer"
	cookieHeader         = "cookie"
	tenantHeader         = "x-ingate-tenant"
	jwtClaimHeaderPrefix = "x-ingate-jwt-claim-"
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
	seen := map[string]struct{}{
		apiKeyHeader:   {},
		cookieHeader:   {},
		consumerHeader: {},
		tenantHeader:   {},
	}
	for _, binding := range route.Bindings {
		for _, policy := range binding.Policies {
			for _, rule := range policy.Rules {
				for _, part := range rule.Key {
					switch {
					case part.Type == config.KeyTypeHeader && part.Name != "":
						seen[part.Name] = struct{}{}
					case part.Type == config.KeyTypeCookie:
						seen[cookieHeader] = struct{}{}
					case part.Type == config.KeyTypeConsumer:
						seen[consumerHeader] = struct{}{}
					case part.Type == config.KeyTypeJWTClaim && part.Name != "":
						seen[jwtClaimHeaderPrefix+part.Name] = struct{}{}
					}
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
		if req.RemoteAddr == "" {
			return "", false
		}
		return clientIP(req.RemoteAddr), true
	case config.KeyTypeHeader:
		value := headerValue(req.Headers, part.Name)
		return value, value != ""
	case config.KeyTypeQuery:
		values, err := url.ParseQuery(rawQuery(req.Path))
		if err != nil {
			return "", false
		}
		value := values.Get(part.Name)
		return value, value != ""
	case config.KeyTypeCookie:
		value := cookieValue(headerValue(req.Headers, cookieHeader), part.Name)
		return value, value != ""
	case config.KeyTypeConsumer:
		value := headerValue(req.Headers, consumerHeader)
		return value, value != ""
	case config.KeyTypeGateway:
		return req.GatewayName, req.GatewayName != ""
	case config.KeyTypeRoute:
		if req.RuleName != "" {
			return req.RouteName + "/" + req.RuleName, req.RouteName != ""
		}
		return req.RouteName, req.RouteName != ""
	case config.KeyTypeRouteRule:
		return req.RuleName, req.RuleName != ""
	case config.KeyTypeAPIKey:
		value := headerValue(req.Headers, apiKeyHeader)
		return value, value != ""
	case config.KeyTypeTenant:
		value := headerValue(req.Headers, tenantHeader)
		return value, value != ""
	case config.KeyTypeJWTClaim:
		value := headerValue(req.Headers, jwtClaimHeaderPrefix+part.Name)
		return value, value != ""
	default:
		return "", false
	}
}

func compositeKey(req Request, parts []config.KeyPart) (string, bool) {
	if len(parts) == 0 {
		return "", false
	}
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value, ok := keyValue(req, part)
		if !ok {
			return "", false
		}
		values = append(values, string(part.Type)+"="+value)
	}
	return strings.Join(values, "|"), true
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

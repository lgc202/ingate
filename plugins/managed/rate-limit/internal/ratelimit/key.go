package ratelimit

import (
	"net/url"
	"strings"

	"github.com/lgc202/ingate/plugins/managed/rate-limit/internal/config"
)

const (
	consumerHeader = "x-ingate-consumer"
	cookieHeader   = "cookie"
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
		values = append(values, part.Type+"="+value)
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
	if i := strings.Index(path, "?"); i >= 0 {
		return path[i+1:]
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

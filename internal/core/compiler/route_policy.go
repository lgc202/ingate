package compiler

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/lgc202/ingate/internal/core/ir"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	routePolicyRequestHeaderRewrite = "request-header-rewrite"
	routePolicyTimeout              = "timeout"
	routePolicyRetry                = "retry"

	routePolicyRequestHeaderRewriteName = "请求 Header 改写"
	routePolicyTimeoutName              = "超时控制"
	routePolicyRetryName                = "失败重试"

	routePolicySourceRoute = "route"

	paramSetHeadersOn        = "setHeadersOn"
	paramHeaderValue         = "value"
	paramRemoveHeadersOn     = "removeHeadersOn"
	paramTimeoutMillis       = "timeoutMillis"
	paramRetryAttempts       = "attempts"
	paramPerTryTimeoutMillis = "perTryTimeoutMillis"
	minRouteTimeoutMillis    = 100
	maxRouteTimeoutMillis    = 300000
	minRetryAttempts         = 1
	maxRetryAttempts         = 5
	minPerTryTimeoutMillis   = 100
	maxPerTryTimeoutMillis   = 60000
)

var headerNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._~-]*$`)

type routePolicyKind string

type routePolicyBinding struct {
	PolicyName string                `json:"policyName"`
	Source     string                `json:"source"`
	Parameters routePolicyParameters `json:"parameters"`
}

type routePolicyParameters map[string]any

type routePolicyApplier struct {
	route resource.Route
	rule  *ir.LogicalRouteRule
}

func (c *gatewayCompiler) applyRoutePolicies(route resource.Route, rule *ir.LogicalRouteRule) error {
	return routePolicyApplier{route: route, rule: rule}.apply()
}

func (a routePolicyApplier) apply() error {
	bindings, err := a.bindings()
	if err != nil {
		return fmt.Errorf("route %q policy bindings: %w", a.route.Name, err)
	}

	for _, binding := range bindings {
		if binding.Source != "" && binding.Source != routePolicySourceRoute {
			return fmt.Errorf("route %q policy %q has unsupported source %q", a.route.Name, binding.PolicyName, binding.Source)
		}

		switch binding.kind() {
		case routePolicyRequestHeaderRewrite:
			if err := a.applyHeaderRewrite(binding.Parameters); err != nil {
				return fmt.Errorf("route %q policy %q: %w", a.route.Name, binding.PolicyName, err)
			}
		case routePolicyTimeout:
			timeoutMillis, err := binding.Parameters.intValue(paramTimeoutMillis, minRouteTimeoutMillis, maxRouteTimeoutMillis)
			if err != nil {
				return fmt.Errorf("route %q policy %q: %w", a.route.Name, binding.PolicyName, err)
			}
			a.rule.TimeoutMillis = timeoutMillis
		case routePolicyRetry:
			retry, err := binding.Parameters.retryPolicy()
			if err != nil {
				return fmt.Errorf("route %q policy %q: %w", a.route.Name, binding.PolicyName, err)
			}
			a.rule.Retry = retry
		default:
			return fmt.Errorf("route %q has unsupported policy %q", a.route.Name, binding.PolicyName)
		}
	}

	return nil
}

func (a routePolicyApplier) bindings() ([]routePolicyBinding, error) {
	if a.route.Annotations == nil || a.route.Annotations[resource.AnnotationRoutePolicyBindings] == "" {
		return nil, nil
	}

	var bindings []routePolicyBinding
	if err := json.Unmarshal([]byte(a.route.Annotations[resource.AnnotationRoutePolicyBindings]), &bindings); err != nil {
		return nil, err
	}
	return bindings, nil
}

func (b routePolicyBinding) kind() routePolicyKind {
	switch b.PolicyName {
	case routePolicyRequestHeaderRewrite, routePolicyRequestHeaderRewriteName:
		return routePolicyRequestHeaderRewrite
	case routePolicyTimeout, routePolicyTimeoutName:
		return routePolicyTimeout
	case routePolicyRetry, routePolicyRetryName:
		return routePolicyRetry
	default:
		return ""
	}
}

func (a routePolicyApplier) applyHeaderRewrite(parameters routePolicyParameters) error {
	setHeaders, err := parameters.headerList(paramSetHeadersOn)
	if err != nil {
		return err
	}
	headerValue := ""
	if len(setHeaders) > 0 {
		value, err := parameters.stringValue(paramHeaderValue)
		if err != nil {
			return err
		}
		headerValue = value
	}
	for _, name := range setHeaders {
		a.rule.RequestHeadersToAdd = append(a.rule.RequestHeadersToAdd, ir.LogicalHeaderValue{
			Name:  name,
			Value: headerValue,
		})
	}

	removeHeaders, err := parameters.headerList(paramRemoveHeadersOn)
	if err != nil {
		return err
	}
	a.rule.RequestHeadersToRemove = append(a.rule.RequestHeadersToRemove, removeHeaders...)
	if len(setHeaders) == 0 && len(removeHeaders) == 0 {
		return fmt.Errorf("at least one header rewrite action is required")
	}

	return nil
}

func (p routePolicyParameters) retryPolicy() (ir.LogicalRetryPolicy, error) {
	attempts, err := p.intValue(paramRetryAttempts, minRetryAttempts, maxRetryAttempts)
	if err != nil {
		return ir.LogicalRetryPolicy{}, err
	}
	perTryTimeoutMillis, err := p.intValue(paramPerTryTimeoutMillis, minPerTryTimeoutMillis, maxPerTryTimeoutMillis)
	if err != nil {
		return ir.LogicalRetryPolicy{}, err
	}

	return ir.LogicalRetryPolicy{
		Attempts:            attempts,
		PerTryTimeoutMillis: perTryTimeoutMillis,
	}, nil
}

func (p routePolicyParameters) stringValue(key string) (string, error) {
	value, ok := p[key]
	if !ok {
		return "", fmt.Errorf("missing parameter %q", key)
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("parameter %q must be a string", key)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("parameter %q must not be empty", key)
	}
	return text, nil
}

func (p routePolicyParameters) intValue(key string, minValue int, maxValue int) (int, error) {
	value, ok := p[key]
	if !ok {
		return 0, fmt.Errorf("missing parameter %q", key)
	}

	var number int
	switch typed := value.(type) {
	case int:
		number = typed
	case int64:
		number = int(typed)
	case float64:
		if math.Trunc(typed) != typed {
			return 0, fmt.Errorf("parameter %q must be an integer", key)
		}
		number = int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, fmt.Errorf("parameter %q must be an integer", key)
		}
		number = parsed
	default:
		return 0, fmt.Errorf("parameter %q must be an integer", key)
	}

	if number < minValue || number > maxValue {
		return 0, fmt.Errorf("parameter %q must be between %d and %d", key, minValue, maxValue)
	}
	return number, nil
}

func (p routePolicyParameters) headerList(key string) ([]string, error) {
	value, ok := p[key]
	if !ok || value == nil {
		return nil, nil
	}

	var items []string
	switch typed := value.(type) {
	case []string:
		items = typed
	case []any:
		items = make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("parameter %q must be a string list", key)
			}
			items = append(items, text)
		}
	case string:
		items = strings.FieldsFunc(typed, func(r rune) bool {
			return r == ',' || r == '，' || r == '、' || r == '\n' || r == '\t' || r == ' '
		})
	default:
		return nil, fmt.Errorf("parameter %q must be a string list", key)
	}

	headers := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		header := strings.ToLower(strings.TrimSpace(item))
		if header == "" {
			continue
		}
		if !headerNamePattern.MatchString(header) {
			return nil, fmt.Errorf("parameter %q contains invalid header %q", key, item)
		}
		if !seen[header] {
			seen[header] = true
			headers = append(headers, header)
		}
	}
	return headers, nil
}

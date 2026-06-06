package dto

import (
	"errors"
	"net/http"
	"strings"
)

const (
	defaultRouteTimeoutMillis = 30000
	minRouteTimeoutMillis     = 100
	maxRouteTimeoutMillis     = 300000
	minRetryAttempts          = 1
	maxRetryAttempts          = 5
	minPerTryTimeoutMillis    = 100
	maxPerTryTimeoutMillis    = 60000
)

// Validate 校验创建 Route 请求
func (r *CreateRouteReq) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return errors.New("路由名称不能为空")
	}

	gatewayIDs, err := routeGatewayIDs(r.GatewayIDs)
	if err != nil {
		return err
	}
	r.GatewayIDs = gatewayIDs

	hostnames, err := routeHostnames(r.Hostnames)
	if err != nil {
		return err
	}
	r.Hostnames = hostnames

	if len(r.Rules) == 0 {
		return errors.New("at least one route rule is required")
	}
	seenRules := make(map[string]struct{}, len(r.Rules))
	for i := range r.Rules {
		if err := r.Rules[i].Validate(); err != nil {
			return err
		}
		if _, ok := seenRules[r.Rules[i].Name]; ok {
			return errors.New("route rule name cannot be duplicated")
		}
		seenRules[r.Rules[i].Name] = struct{}{}
	}
	return nil
}

// Validate 校验更新 Route 请求
func (r *UpdateRouteReq) Validate() error {
	if r.Version == "" {
		return errors.New("路由版本不能为空")
	}
	return r.CreateRouteReq.Validate()
}

// EnabledValue 返回创建和更新请求中的 Route 启停值
func (r CreateRouteReq) EnabledValue() bool {
	return r.Enabled == nil || *r.Enabled
}

func routeGatewayIDs(items []string) ([]string, error) {
	if len(items) == 0 {
		return nil, errors.New("at least one gateway is required")
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item)
		if id == "" {
			return nil, errors.New("gateway id cannot be empty")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func routeHostnames(items []string) ([]string, error) {
	hostnames := make([]string, 0, len(items))
	for _, item := range items {
		hostname := strings.TrimSpace(strings.ToLower(item))
		if hostname == "" {
			return nil, errors.New("route hostname cannot be empty")
		}
		if !validHostname(hostname) {
			return nil, errors.New("route hostname is invalid")
		}
		hostnames = append(hostnames, hostname)
	}
	return hostnames, nil
}

// Validate 校验并规整单条 Route 规则
func (r *RouteRule) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return errors.New("route rule name is required")
	}

	r.PathPrefix = strings.TrimSpace(r.PathPrefix)
	if r.PathPrefix == "" || !strings.HasPrefix(r.PathPrefix, "/") {
		return errors.New("route rule pathPrefix must start with /")
	}

	for _, method := range r.Methods {
		if !validHTTPMethod(method) {
			return errors.New("route rule method is invalid")
		}
	}

	headers, err := r.headerMatches()
	if err != nil {
		return err
	}
	r.Headers = headers

	targets, err := r.targetServices()
	if err != nil {
		return err
	}
	r.Targets = targets
	return r.validateRouteNativePolicies()
}

func (r *RouteRule) headerMatches() ([]HeaderMatchReq, error) {
	headers := make([]HeaderMatchReq, 0, len(r.Headers))
	for _, header := range r.Headers {
		name := strings.TrimSpace(strings.ToLower(header.Name))
		value := strings.TrimSpace(header.Value)
		if name == "" {
			return nil, errors.New("route rule header name is required")
		}
		if value == "" {
			return nil, errors.New("route rule header value is required")
		}
		headers = append(headers, HeaderMatchReq{Name: name, Value: value})
	}
	return headers, nil
}

func (r *RouteRule) targetServices() ([]RouteTarget, error) {
	targets := r.Targets
	if len(targets) == 0 {
		return nil, errors.New("at least one route target is required")
	}
	seenTargets := map[string]struct{}{}
	values := make([]RouteTarget, 0, len(targets))
	for _, target := range targets {
		upstreamID := strings.TrimSpace(target.UpstreamID)
		if upstreamID == "" {
			return nil, errors.New("target upstream id is required")
		}
		if _, ok := seenTargets[upstreamID]; ok {
			return nil, errors.New("target upstream cannot be duplicated")
		}
		if target.Weight < 1 || target.Weight > 100 {
			return nil, errors.New("target upstream weight must be between 1 and 100")
		}
		seenTargets[upstreamID] = struct{}{}
		values = append(values, RouteTarget{UpstreamID: upstreamID, Weight: target.Weight})
	}
	return values, nil
}

func (r *RouteRule) validateRouteNativePolicies() error {
	totalTimeoutMillis := defaultRouteTimeoutMillis

	if r.RequestHeaderModifier != nil {
		if err := r.RequestHeaderModifier.Validate(); err != nil {
			return err
		}
	}
	if r.ResponseHeaderModifier != nil {
		if err := r.ResponseHeaderModifier.Validate(); err != nil {
			return err
		}
	}
	if r.Timeout != nil {
		if r.Timeout.RequestMillis < minRouteTimeoutMillis || r.Timeout.RequestMillis > maxRouteTimeoutMillis {
			return errors.New("route timeout is out of range")
		}
		totalTimeoutMillis = r.Timeout.RequestMillis
	}
	if r.Retry != nil {
		if r.Retry.Attempts < minRetryAttempts || r.Retry.Attempts > maxRetryAttempts {
			return errors.New("route retry attempts is out of range")
		}
		if r.Retry.PerTryTimeoutMillis < minPerTryTimeoutMillis || r.Retry.PerTryTimeoutMillis > maxPerTryTimeoutMillis {
			return errors.New("route retry per-try timeout is out of range")
		}
		if r.Retry.PerTryTimeoutMillis > totalTimeoutMillis {
			return errors.New("retry per-try timeout must be less than or equal to route total timeout")
		}
	}

	return nil
}

// Validate 校验并规整 Header 改写配置
func (r *HeaderModifierReq) Validate() error {
	setHeaders := make([]HeaderValueReq, 0, len(r.Set))
	for _, header := range r.Set {
		name := strings.TrimSpace(strings.ToLower(header.Name))
		value := strings.TrimSpace(header.Value)
		if name == "" {
			return errors.New("header name is required")
		}
		if value == "" {
			return errors.New("header value is required")
		}
		setHeaders = append(setHeaders, HeaderValueReq{Name: name, Value: value})
	}

	removeHeaders := lowerNonEmptyStrings(r.Remove)
	if len(setHeaders) == 0 && len(removeHeaders) == 0 {
		return errors.New("at least one header rewrite action is required")
	}
	r.Set = setHeaders
	r.Remove = removeHeaders
	return nil
}

func lowerNonEmptyStrings(items []string) []string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(strings.ToLower(item))
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func validHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func validHostname(hostname string) bool {
	if len(hostname) > 2 && hostname[:2] == "*." {
		hostname = hostname[2:]
	}
	hostname = strings.ToLower(hostname)
	if hostname == "" || len(hostname) > 253 {
		return false
	}
	for label := range strings.SplitSeq(hostname, ".") {
		if !validDNSLabel(label) {
			return false
		}
	}
	return true
}

func validDNSLabel(label string) bool {
	if label == "" || len(label) > 63 {
		return false
	}
	for i, r := range label {
		valid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-'
		if !valid {
			return false
		}
		if (i == 0 || i == len(label)-1) && r == '-' {
			return false
		}
	}
	return true
}

// Validate 校验控制台提交的 Route 启停请求体
func (r SetRouteEnabledReq) Validate() error {
	if r.Enabled == nil {
		return errors.New("enabled is required")
	}
	return nil
}

// Value 返回已校验的启停值
func (r SetRouteEnabledReq) Value() bool {
	return *r.Enabled
}

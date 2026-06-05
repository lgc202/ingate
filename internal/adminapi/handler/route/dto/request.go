package dto

import (
	"fmt"
	"strconv"
	"strings"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

const defaultRouteTimeoutMillis = 30000

// Resource 将已校验的控制台请求体转换为后端声明式 Route 资源
func (r RouteRequest) Resource() (*resource.Route, error) {
	rule, err := r.rule()
	if err != nil {
		return nil, err
	}

	return &resource.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindRoute),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            r.name(),
			ResourceVersion: strings.TrimSpace(r.Version),
			Annotations: map[string]string{
				resource.AnnotationRouteEnabled: strconv.FormatBool(r.Enabled),
			},
		},
		Spec: resource.RouteSpec{
			ParentRefs: r.gatewayNames(),
			Hostnames:  r.hostnames(),
			Rules:      []resource.RouteRule{rule},
		},
	}, nil
}

func (r RouteRequest) rule() (resource.RouteRule, error) {
	rule := resource.RouteRule{
		PathPrefix:   strings.TrimSpace(r.Path),
		Methods:      r.methods(),
		Headers:      []resource.HeaderMatch{},
		UpstreamRefs: r.upstreamRefs(),
	}
	for _, binding := range r.PolicyBindings {
		if err := binding.applyToRouteRule(&rule); err != nil {
			return resource.RouteRule{}, err
		}
	}
	return rule, nil
}

// Validate 校验控制台提交的 Route 请求体
func (r RouteRequest) Validate() error {
	if id := strings.TrimSpace(r.ID); id != "" {
		if errs := validation.IsDNS1123Label(id); len(errs) > 0 {
			return apierrors.NewBadRequest("route id must be a valid DNS label")
		}
	}

	path := strings.TrimSpace(r.Path)
	if path == "" || !strings.HasPrefix(path, "/") {
		return apierrors.NewBadRequest("route path must start with /")
	}
	for _, method := range r.Methods {
		if !validHTTPMethod(method) {
			return apierrors.NewBadRequest("route method is invalid")
		}
	}
	if len(r.GatewayNames) == 0 {
		return apierrors.NewBadRequest("at least one gateway is required")
	}
	for _, gatewayName := range r.GatewayNames {
		if errs := validation.IsDNS1123Label(strings.TrimSpace(gatewayName)); len(errs) > 0 {
			return apierrors.NewBadRequest("gateway name must be a valid DNS label")
		}
	}
	targets := r.targetServices()
	if len(targets) == 0 {
		return apierrors.NewBadRequest("at least one target service is required")
	}
	seenTargets := map[string]struct{}{}
	for _, target := range targets {
		name := strings.TrimSpace(target.Name)
		if errs := validation.IsDNS1123Label(name); len(errs) > 0 {
			return apierrors.NewBadRequest("target service name must be a valid DNS label")
		}
		if _, ok := seenTargets[name]; ok {
			return apierrors.NewBadRequest("target service cannot be duplicated")
		}
		seenTargets[name] = struct{}{}
		if target.Weight < 1 || target.Weight > 1000 {
			return apierrors.NewBadRequest("target service weight must be between 1 and 1000")
		}
	}
	for _, hostname := range r.Hostnames {
		hostname = strings.TrimSpace(strings.ToLower(hostname))
		if hostname == "" {
			return apierrors.NewBadRequest("route hostname cannot be empty")
		}
		if !validHostname(hostname) {
			return apierrors.NewBadRequest("route hostname is invalid")
		}
	}
	if err := r.validatePolicyBindings(); err != nil {
		return err
	}
	return nil
}

func (r RouteRequest) validatePolicyBindings() error {
	totalTimeoutMillis := defaultRouteTimeoutMillis
	perTryTimeoutMillis := 0

	for _, binding := range r.PolicyBindings {
		if binding.Source != routePolicySourceNative {
			return apierrors.NewBadRequest("route policy source is invalid")
		}

		switch binding.Capability {
		case routePolicyRequestHeaderModifier:
			if err := binding.validateHeaderRewrite(); err != nil {
				return err
			}
		case routePolicyTimeout:
			timeoutMillis, err := binding.intParameter(paramTimeoutMillis, minRouteTimeoutMillis, maxRouteTimeoutMillis)
			if err != nil {
				return err
			}
			totalTimeoutMillis = timeoutMillis
		case routePolicyRetry:
			if _, err := binding.intParameter(paramRetryAttempts, minRetryAttempts, maxRetryAttempts); err != nil {
				return err
			}
			value, err := binding.intParameter(paramPerTryTimeoutMillis, minPerTryTimeoutMillis, maxPerTryTimeoutMillis)
			if err != nil {
				return err
			}
			perTryTimeoutMillis = value
		default:
			return apierrors.NewBadRequest("route policy is unsupported")
		}
	}

	if perTryTimeoutMillis > totalTimeoutMillis {
		return apierrors.NewBadRequest("retry per-try timeout must be less than or equal to route total timeout")
	}
	return nil
}

func (b PolicyBindingRequest) validateHeaderRewrite() error {
	setHeaders, err := b.stringListParameter(paramSetHeadersOn)
	if err != nil {
		return err
	}
	removeHeaders, err := b.stringListParameter(paramRemoveHeadersOn)
	if err != nil {
		return err
	}
	if len(setHeaders) == 0 && len(removeHeaders) == 0 {
		return apierrors.NewBadRequest("at least one header rewrite action is required")
	}
	if len(setHeaders) > 0 && strings.TrimSpace(b.stringParameter(paramHeaderValue)) == "" {
		return apierrors.NewBadRequest("header value is required")
	}
	if len(setHeaders) == 0 && strings.TrimSpace(b.stringParameter(paramHeaderValue)) != "" {
		return apierrors.NewBadRequest("header name is required")
	}
	return nil
}

func (b PolicyBindingRequest) applyToRouteRule(rule *resource.RouteRule) error {
	if b.Source != routePolicySourceNative {
		return apierrors.NewBadRequest("route policy source is invalid")
	}

	switch b.Capability {
	case routePolicyRequestHeaderModifier:
		return b.applyHeaderRewrite(rule)
	case routePolicyTimeout:
		timeoutMillis, err := b.intParameter(paramTimeoutMillis, minRouteTimeoutMillis, maxRouteTimeoutMillis)
		if err != nil {
			return err
		}
		rule.Timeout = &resource.RouteTimeout{RequestMillis: timeoutMillis}
	case routePolicyRetry:
		attempts, err := b.intParameter(paramRetryAttempts, minRetryAttempts, maxRetryAttempts)
		if err != nil {
			return err
		}
		perTryTimeoutMillis, err := b.intParameter(paramPerTryTimeoutMillis, minPerTryTimeoutMillis, maxPerTryTimeoutMillis)
		if err != nil {
			return err
		}
		rule.Retry = &resource.RouteRetry{
			Attempts:            attempts,
			PerTryTimeoutMillis: perTryTimeoutMillis,
		}
	default:
		return apierrors.NewBadRequest("route policy is unsupported")
	}
	return nil
}

func (b PolicyBindingRequest) applyHeaderRewrite(rule *resource.RouteRule) error {
	setHeaders, err := b.stringListParameter(paramSetHeadersOn)
	if err != nil {
		return err
	}
	removeHeaders, err := b.stringListParameter(paramRemoveHeadersOn)
	if err != nil {
		return err
	}

	modifier := resource.HeaderModifier{
		Set:    make([]resource.HeaderValue, 0, len(setHeaders)),
		Remove: removeHeaders,
	}
	value := b.stringParameter(paramHeaderValue)
	for _, header := range setHeaders {
		modifier.Set = append(modifier.Set, resource.HeaderValue{
			Name:  header,
			Value: value,
		})
	}
	rule.Filters = append(rule.Filters, resource.RouteFilter{
		Type:                  resource.RouteFilterRequestHeaderModifier,
		RequestHeaderModifier: &modifier,
	})
	return nil
}

func (b PolicyBindingRequest) intParameter(key string, minValue int, maxValue int) (int, error) {
	value, ok := b.Parameters[key]
	if !ok {
		return 0, apierrors.NewBadRequest("route policy parameter is required")
	}

	var text string
	switch item := value.(type) {
	case string:
		text = strings.TrimSpace(item)
	case float64:
		text = strconv.FormatInt(int64(item), 10)
	default:
		return 0, apierrors.NewBadRequest("route policy parameter must be a number")
	}
	if text == "" {
		return 0, apierrors.NewBadRequest("route policy parameter is required")
	}

	number, err := strconv.Atoi(text)
	if err != nil {
		return 0, apierrors.NewBadRequest("route policy parameter must be a number")
	}
	if number < minValue || number > maxValue {
		return 0, apierrors.NewBadRequest("route policy parameter is out of range")
	}
	return number, nil
}

func (b PolicyBindingRequest) stringParameter(key string) string {
	value, ok := b.Parameters[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func (b PolicyBindingRequest) stringListParameter(key string) ([]string, error) {
	value, ok := b.Parameters[key]
	if !ok {
		return nil, nil
	}

	switch items := value.(type) {
	case []string:
		return nonEmptyStrings(items), nil
	case []any:
		values := make([]string, 0, len(items))
		for _, item := range items {
			text, ok := item.(string)
			if !ok {
				return nil, apierrors.NewBadRequest("route policy parameter must be a string list")
			}
			values = append(values, text)
		}
		return nonEmptyStrings(values), nil
	case string:
		if strings.TrimSpace(items) == "" {
			return nil, nil
		}
		return nonEmptyStrings(strings.FieldsFunc(items, func(r rune) bool {
			return r == ',' || r == '，' || r == '、'
		})), nil
	default:
		return nil, apierrors.NewBadRequest("route policy parameter must be a string list")
	}
}

func nonEmptyStrings(items []string) []string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(strings.ToLower(item))
		if item != "" {
			values = append(values, item)
		}
	}
	return values
}

func validHTTPMethod(method HTTPMethod) bool {
	switch method {
	case HTTPMethodGET, HTTPMethodPOST, HTTPMethodPUT, HTTPMethodPATCH, HTTPMethodDELETE:
		return true
	default:
		return false
	}
}

func validHostname(hostname string) bool {
	hostname = strings.TrimPrefix(hostname, "*.")
	return len(validation.IsDNS1123Subdomain(hostname)) == 0
}

func (r RouteRequest) name() string {
	if id := strings.TrimSpace(r.ID); id != "" {
		return id
	}

	method := "any"
	if len(r.Methods) > 0 {
		method = strings.ToLower(string(r.Methods[0]))
	}
	targetName := "route"
	if targets := r.targetServices(); len(targets) > 0 {
		targetName = targets[0].Name
	}
	return dnsLabel(fmt.Sprintf("%s-%s-%s", targetName, method, r.Path))
}

func dnsLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	builder := strings.Builder{}
	previousHyphen := false
	for _, item := range value {
		valid := (item >= 'a' && item <= 'z') || (item >= '0' && item <= '9')
		if valid {
			builder.WriteRune(item)
			previousHyphen = false
			continue
		}
		if !previousHyphen {
			builder.WriteByte('-')
			previousHyphen = true
		}
	}

	name := strings.Trim(builder.String(), "-")
	if name == "" {
		name = "route"
	}
	if len(name) > validation.DNS1123LabelMaxLength {
		name = strings.Trim(name[:validation.DNS1123LabelMaxLength], "-")
	}
	if name == "" {
		return "route"
	}
	return name
}

func (r RouteRequest) methods() []string {
	methods := make([]string, 0, len(r.Methods))
	for _, method := range r.Methods {
		methods = append(methods, string(method))
	}
	return methods
}

func (r RouteRequest) gatewayNames() []string {
	names := make([]string, 0, len(r.GatewayNames))
	for _, name := range r.GatewayNames {
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func (r RouteRequest) hostnames() []string {
	hostnames := make([]string, 0, len(r.Hostnames))
	for _, hostname := range r.Hostnames {
		hostname = strings.TrimSpace(strings.ToLower(hostname))
		if hostname != "" {
			hostnames = append(hostnames, hostname)
		}
	}
	return hostnames
}

func (r RouteRequest) targetServices() []TargetService {
	if len(r.Targets) > 0 {
		targets := make([]TargetService, 0, len(r.Targets))
		for _, target := range r.Targets {
			name := strings.TrimSpace(target.Name)
			if name != "" {
				targets = append(targets, TargetService{Name: name, Weight: target.Weight})
			}
		}
		return targets
	}

	serviceName := strings.TrimSpace(r.ServiceName)
	if serviceName == "" {
		return nil
	}
	return []TargetService{{Name: serviceName, Weight: 100}}
}

func (r RouteRequest) upstreamRefs() []resource.UpstreamRef {
	targets := r.targetServices()
	refs := make([]resource.UpstreamRef, 0, len(targets))
	for _, target := range targets {
		refs = append(refs, resource.UpstreamRef{
			Name:   strings.TrimSpace(target.Name),
			Weight: target.Weight,
		})
	}
	return refs
}

// Validate 校验控制台提交的 Route 启停请求体
func (r EnabledRequest) Validate() error {
	if r.Enabled == nil {
		return apierrors.NewBadRequest("enabled is required")
	}
	return nil
}

// Value 返回已校验的启停值
func (r EnabledRequest) Value() bool {
	return *r.Enabled
}

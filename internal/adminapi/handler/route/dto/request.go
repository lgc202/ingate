package dto

import (
	"encoding/json"
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
	policyBindings, err := json.Marshal(r.PolicyBindings)
	if err != nil {
		return nil, fmt.Errorf("marshal route policy bindings: %w", err)
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
				resource.AnnotationRouteEnabled:        strconv.FormatBool(r.Enabled),
				resource.AnnotationRoutePolicyBindings: string(policyBindings),
			},
		},
		Spec: resource.RouteSpec{
			ParentRefs: r.gatewayNames(),
			Hostnames:  r.hostnames(),
			Rules: []resource.RouteRule{{
				PathPrefix:    strings.TrimSpace(r.Path),
				Methods:       r.methods(),
				TimeoutMillis: defaultRouteTimeoutMillis,
				Headers:       []resource.HeaderMatch{},
				UpstreamRefs: []resource.UpstreamRef{{
					Name:   strings.TrimSpace(r.ServiceName),
					Weight: 100,
				}},
			}},
		},
	}, nil
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
	if errs := validation.IsDNS1123Label(strings.TrimSpace(r.ServiceName)); len(errs) > 0 {
		return apierrors.NewBadRequest("service name must be a valid DNS label")
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
	return nil
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
	return dnsLabel(fmt.Sprintf("%s-%s-%s", r.ServiceName, method, r.Path))
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

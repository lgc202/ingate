package dto

import (
	"encoding/json"
	"net/netip"
	"strconv"
	"strings"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// Resource 将已校验的控制台请求体转换为后端声明式 Upstream 资源
func (r UpstreamRequest) Resource() (*resource.Upstream, error) {
	endpoints, err := r.endpointAnnotations()
	if err != nil {
		return nil, err
	}
	healthCheck, err := r.healthCheckAnnotation()
	if err != nil {
		return nil, err
	}

	return &resource.Upstream{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindUpstream),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            strings.TrimSpace(r.Name),
			ResourceVersion: strings.TrimSpace(r.Version),
			Annotations: map[string]string{
				resource.AnnotationUpstreamServiceType:       string(r.Type),
				resource.AnnotationUpstreamLoadBalancePolicy: string(r.LoadBalancePolicy),
				resource.AnnotationUpstreamEndpoints:         endpoints,
				resource.AnnotationUpstreamHealthCheck:       healthCheck,
			},
		},
		Spec: resource.UpstreamSpec{
			Endpoints: r.resourceEndpoints(),
		},
	}, nil
}

// Validate 校验控制台提交的 Upstream 请求体
func (r UpstreamRequest) Validate() error {
	name := strings.TrimSpace(r.Name)
	if name == "" {
		return apierrors.NewBadRequest("service name is required")
	}
	if errs := validation.IsDNS1123Label(name); len(errs) > 0 {
		return apierrors.NewBadRequest("service name must be a valid DNS label")
	}
	if !validServiceType(r.Type) {
		return apierrors.NewBadRequest("service type is invalid")
	}
	if !validLoadBalancePolicy(r.LoadBalancePolicy) {
		return apierrors.NewBadRequest("load balance policy is invalid")
	}
	if len(r.Endpoints) == 0 {
		return apierrors.NewBadRequest("at least one service endpoint is required")
	}

	enabledEndpoints := 0
	for _, endpoint := range r.Endpoints {
		if err := endpoint.Validate(); err != nil {
			return err
		}
		if endpoint.Enabled {
			enabledEndpoints++
		}
	}
	if enabledEndpoints == 0 {
		return apierrors.NewBadRequest("at least one service endpoint must be enabled")
	}

	return r.validateHealthCheck()
}

// Validate 校验控制台提交的服务端点
func (r EndpointRequest) Validate() error {
	address := strings.TrimSpace(r.Address)
	if address == "" {
		return apierrors.NewBadRequest("endpoint address is required")
	}
	if !validEndpointAddress(address) {
		return apierrors.NewBadRequest("endpoint address is invalid")
	}

	port, err := strconv.Atoi(strings.TrimSpace(r.Port))
	if err != nil {
		return apierrors.NewBadRequest("endpoint port must be a number")
	}
	if port < 1 || port > 65535 {
		return apierrors.NewBadRequest("endpoint port must be between 1 and 65535")
	}

	weight, err := strconv.Atoi(strings.TrimSpace(r.Weight))
	if err != nil {
		return apierrors.NewBadRequest("endpoint weight must be a number")
	}
	if weight < 0 || weight > 1000 {
		return apierrors.NewBadRequest("endpoint weight must be between 0 and 1000")
	}

	return nil
}

func validServiceType(value ServiceType) bool {
	switch value {
	case ServiceTypeApplication, ServiceTypeModel, ServiceTypeAgent, ServiceTypeMCP:
		return true
	default:
		return false
	}
}

func validLoadBalancePolicy(value LoadBalancePolicy) bool {
	switch value {
	case LoadBalancePolicyRoundRobin, LoadBalancePolicyLeastRequest, LoadBalancePolicyRandom:
		return true
	default:
		return false
	}
}

func validEndpointAddress(address string) bool {
	if _, err := netip.ParseAddr(address); err == nil {
		return true
	}
	return len(validation.IsDNS1123Subdomain(strings.ToLower(address))) == 0
}

func (r UpstreamRequest) validateHealthCheck() error {
	if !r.HealthCheckEnabled {
		return nil
	}
	if !strings.HasPrefix(strings.TrimSpace(r.HealthCheckPath), "/") {
		return apierrors.NewBadRequest("health check path must start with /")
	}

	interval, err := strconv.Atoi(strings.TrimSpace(r.HealthCheckIntervalSeconds))
	if err != nil {
		return apierrors.NewBadRequest("health check interval must be a number")
	}
	if interval < 1 || interval > 300 {
		return apierrors.NewBadRequest("health check interval must be between 1 and 300")
	}

	timeout, err := strconv.Atoi(strings.TrimSpace(r.HealthCheckTimeoutSeconds))
	if err != nil {
		return apierrors.NewBadRequest("health check timeout must be a number")
	}
	if timeout < 1 || timeout > 60 || timeout >= interval {
		return apierrors.NewBadRequest("health check timeout must be between 1 and 60 and less than interval")
	}

	return nil
}

func (r UpstreamRequest) resourceEndpoints() []resource.Endpoint {
	endpoints := make([]resource.Endpoint, 0, len(r.Endpoints))
	for _, endpoint := range r.Endpoints {
		if !endpoint.Enabled {
			continue
		}
		port, _ := strconv.Atoi(strings.TrimSpace(endpoint.Port))
		endpoints = append(endpoints, resource.Endpoint{
			Address: strings.TrimSpace(endpoint.Address),
			Port:    port,
		})
	}
	return endpoints
}

func (r UpstreamRequest) endpointAnnotations() (string, error) {
	endpoints := make([]EndpointRequest, 0, len(r.Endpoints))
	for _, endpoint := range r.Endpoints {
		endpoints = append(endpoints, EndpointRequest{
			ID:      strings.TrimSpace(endpoint.ID),
			Address: strings.TrimSpace(endpoint.Address),
			Port:    strings.TrimSpace(endpoint.Port),
			Weight:  strings.TrimSpace(endpoint.Weight),
			Enabled: endpoint.Enabled,
		})
	}
	data, err := json.Marshal(endpoints)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r UpstreamRequest) healthCheckAnnotation() (string, error) {
	interval, _ := strconv.Atoi(strings.TrimSpace(r.HealthCheckIntervalSeconds))
	timeout, _ := strconv.Atoi(strings.TrimSpace(r.HealthCheckTimeoutSeconds))
	data, err := json.Marshal(healthCheckAnnotation{
		Enabled:         r.HealthCheckEnabled,
		Path:            strings.TrimSpace(r.HealthCheckPath),
		IntervalSeconds: interval,
		TimeoutSeconds:  timeout,
	})
	if err != nil {
		return "", err
	}
	return string(data), nil
}

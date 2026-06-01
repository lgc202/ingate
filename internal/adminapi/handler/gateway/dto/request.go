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

// Resource 将已校验的控制台请求体转换为后端声明式 Gateway 资源
func (r GatewayRequest) Resource() (*resource.Gateway, error) {
	annotations := map[string]string{
		resource.AnnotationGatewayEnabled: strconv.FormatBool(r.enabled()),
	}

	description := strings.TrimSpace(r.Description)
	if description != "" {
		annotations[resource.AnnotationGatewayDescription] = description
	}

	hostnames := r.hostnames()
	if len(hostnames) > 0 {
		data, err := json.Marshal(hostnames)
		if err != nil {
			return nil, fmt.Errorf("marshal gateway hostnames: %w", err)
		}
		annotations[resource.AnnotationGatewayHostnames] = string(data)
	}

	return &resource.Gateway{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindGateway),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            strings.TrimSpace(r.Name),
			ResourceVersion: strings.TrimSpace(r.Version),
			Annotations:     annotations,
		},
		Spec: resource.GatewaySpec{
			Listeners: r.listeners(hostnames),
		},
	}, nil
}

// Validate 校验控制台提交的 Gateway 请求体
func (r GatewayRequest) Validate() error {
	name := strings.TrimSpace(r.Name)
	if name == "" {
		return apierrors.NewBadRequest("gateway name is required")
	}
	if errs := validation.IsDNS1123Label(name); len(errs) > 0 {
		return apierrors.NewBadRequest("gateway name must be a valid DNS label")
	}
	if len(r.Listeners) == 0 {
		return apierrors.NewBadRequest("at least one listener is required")
	}

	ports := map[int]struct{}{}
	for _, listener := range r.Listeners {
		protocol := strings.TrimSpace(listener.Protocol)
		if protocol != resource.ListenerProtocolHTTP && protocol != resource.ListenerProtocolHTTPS {
			return apierrors.NewBadRequest("listener protocol must be HTTP or HTTPS")
		}

		port, err := strconv.Atoi(strings.TrimSpace(listener.Port))
		if err != nil {
			return apierrors.NewBadRequest("listener port must be a number")
		}
		if port < 1 || port > 65535 {
			return apierrors.NewBadRequest("listener port must be between 1 and 65535")
		}
		if _, ok := ports[port]; ok {
			return apierrors.NewBadRequest("listener ports cannot be duplicated")
		}
		ports[port] = struct{}{}
	}

	for _, hostname := range r.Hostnames {
		hostname = strings.TrimSpace(strings.ToLower(hostname))
		if hostname == "" {
			return apierrors.NewBadRequest("gateway hostname cannot be empty")
		}
		if !validHostname(hostname) {
			return apierrors.NewBadRequest("gateway hostname is invalid")
		}
	}
	return nil
}

func validHostname(hostname string) bool {
	hostname = strings.TrimPrefix(hostname, "*.")
	return len(validation.IsDNS1123Subdomain(hostname)) == 0
}

func (r GatewayRequest) enabled() bool {
	if r.Enabled == nil {
		return true
	}
	return *r.Enabled
}

func (r GatewayRequest) hostnames() []string {
	hostnames := make([]string, 0, len(r.Hostnames))
	for _, hostname := range r.Hostnames {
		hostname = strings.TrimSpace(strings.ToLower(hostname))
		if hostname != "" {
			hostnames = append(hostnames, hostname)
		}
	}
	return hostnames
}

func (r GatewayRequest) listeners(hostnames []string) []resource.Listener {
	listeners := make([]resource.Listener, 0, len(r.Listeners))
	hostname := ""
	if len(hostnames) == 1 {
		hostname = hostnames[0]
	}

	for _, listener := range r.Listeners {
		port, _ := strconv.Atoi(strings.TrimSpace(listener.Port))
		name := strings.TrimSpace(listener.ID)
		protocol := strings.TrimSpace(listener.Protocol)
		if name == "" {
			name = fmt.Sprintf("%s-%d", strings.ToLower(protocol), port)
		}
		listeners = append(listeners, resource.Listener{
			Name:     name,
			Protocol: protocol,
			Port:     port,
			Hostname: hostname,
		})
	}
	return listeners
}

// Validate 校验控制台提交的 Gateway 启停请求体
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

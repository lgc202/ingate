package dto

import (
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/pkg/apperror"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// Validate 校验并归一化创建 Gateway 请求
func (r *CreateGatewayReq) Validate() error {
	if r.Name == "" {
		return apperror.NewBadRequest("gateway name is required")
	}
	if errs := validation.IsDNS1123Label(r.Name); len(errs) > 0 {
		return apperror.NewBadRequest("gateway name must be a valid DNS label")
	}
	r.RuntimeGroup = normalizeRuntimeGroup(r.RuntimeGroup)
	return validateGatewaySpecRequest(r.Listeners, r.HostBindings)
}

// Validate 校验并归一化更新 Gateway 请求
func (r *UpdateGatewayReq) Validate() error {
	if r.Version == "" {
		return apperror.NewBadRequest("gateway version is required")
	}
	r.RuntimeGroup = normalizeRuntimeGroup(r.RuntimeGroup)
	return validateGatewaySpecRequest(r.Listeners, r.HostBindings)
}

// Validate 校验 Gateway 启停请求
func (r *SetGatewayEnabledReq) Validate() error {
	if r.Enabled == nil {
		return apperror.NewBadRequest("enabled is required")
	}
	return nil
}

// Value 返回已校验的启停值
func (r *SetGatewayEnabledReq) Value() bool {
	return *r.Enabled
}

func validateGatewaySpecRequest(listeners []GatewayListenerReq, bindings []GatewayHostBindingReq) error {
	if len(listeners) == 0 {
		return apperror.NewBadRequest("at least one listener is required")
	}

	listenerProtocols := make(map[string]string, len(listeners))
	ports := map[int]struct{}{}
	for i := range listeners {
		listener := &listeners[i]
		if listener.Name == "" {
			return apperror.NewBadRequest("listener name is required")
		}
		if _, ok := listenerProtocols[listener.Name]; ok {
			return apperror.NewBadRequest(fmt.Sprintf("listener %q is duplicated", listener.Name))
		}
		if listener.Protocol != string(resource.ListenerProtocolHTTP) && listener.Protocol != string(resource.ListenerProtocolHTTPS) {
			return apperror.NewBadRequest("listener protocol must be HTTP or HTTPS")
		}
		if listener.Port < 1 || listener.Port > 65535 {
			return apperror.NewBadRequest("listener port must be between 1 and 65535")
		}
		if _, ok := ports[listener.Port]; ok {
			return apperror.NewBadRequest("listener ports cannot be duplicated")
		}
		listenerProtocols[listener.Name] = listener.Protocol
		ports[listener.Port] = struct{}{}
	}

	catchAllCount := 0
	httpsListenersWithTLS := map[string]struct{}{}
	for i := range bindings {
		binding := &bindings[i]
		if binding.Hostname == "" {
			catchAllCount++
			if catchAllCount > 1 {
				return apperror.NewBadRequest("only one catch-all host binding is allowed")
			}
		} else if !validHostname(binding.Hostname) {
			return apperror.NewBadRequest("gateway hostname is invalid")
		}

		if len(binding.ListenerRefs) == 0 {
			return apperror.NewBadRequest("host binding listenerRefs is required")
		}
		hasHTTPS := false
		for j := range binding.ListenerRefs {
			listenerRef := binding.ListenerRefs[j]
			if listenerRef == "" {
				return apperror.NewBadRequest("host binding listenerRef cannot be empty")
			}
			protocol, ok := listenerProtocols[listenerRef]
			if !ok {
				return apperror.NewBadRequest(fmt.Sprintf("host binding references unknown listener %q", listenerRef))
			}
			if protocol == string(resource.ListenerProtocolHTTPS) {
				hasHTTPS = true
			}
		}

		certificateRef := ""
		if binding.TLS != nil {
			certificateRef = binding.TLS.CertificateRef
		}
		if hasHTTPS && certificateRef == "" {
			return apperror.NewBadRequest("HTTPS host binding certificateRef is required")
		}
		if !hasHTTPS && certificateRef != "" {
			return apperror.NewBadRequest("HTTP host binding cannot set certificateRef")
		}
		if hasHTTPS {
			for _, listenerRef := range binding.ListenerRefs {
				if listenerProtocols[listenerRef] == string(resource.ListenerProtocolHTTPS) {
					httpsListenersWithTLS[listenerRef] = struct{}{}
				}
			}
		}
	}

	for name, protocol := range listenerProtocols {
		if protocol != string(resource.ListenerProtocolHTTPS) {
			continue
		}
		if _, ok := httpsListenersWithTLS[name]; !ok {
			return apperror.NewBadRequest(fmt.Sprintf("HTTPS listener %q must be referenced by a TLS host binding", name))
		}
	}
	return nil
}

func normalizeRuntimeGroup(runtimeGroup string) string {
	if runtimeGroup == "" {
		return "default"
	}
	return runtimeGroup
}

func validHostname(hostname string) bool {
	if len(hostname) > 2 && hostname[:2] == "*." {
		hostname = hostname[2:]
	}
	return len(validation.IsDNS1123Subdomain(hostname)) == 0
}

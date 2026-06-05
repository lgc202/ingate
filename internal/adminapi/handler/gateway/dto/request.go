package dto

import (
	"errors"
	"fmt"
	"strings"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Validate 校验创建 Gateway 请求
func (r *CreateGatewayReq) Validate() error {
	return r.GatewayConfig.Validate()
}

// Validate 校验更新 Gateway 请求
func (r *UpdateGatewayReq) Validate() error {
	if r.Version == "" {
		return errors.New("gateway version is required")
	}
	return r.GatewayConfig.Validate()
}

// Validate 校验 Gateway 配置请求
func (r *GatewayConfig) Validate() error {
	r.DisplayName = strings.TrimSpace(r.DisplayName)
	if r.DisplayName == "" {
		return errors.New("gateway displayName is required")
	}
	if r.RuntimeGroup == "" {
		return errors.New("gateway runtimeGroup is required")
	}
	return validateGatewayConfig(r.Listeners, r.HostBindings)
}

// Validate 校验 Gateway 启停请求
func (r *SetGatewayEnabledReq) Validate() error {
	if r.Enabled == nil {
		return errors.New("enabled is required")
	}
	return nil
}

// Value 返回已校验的启停值
func (r *SetGatewayEnabledReq) Value() bool {
	return *r.Enabled
}

func validateGatewayConfig(listeners []GatewayListener, bindings []GatewayHostBinding) error {
	if len(listeners) == 0 {
		return errors.New("at least one listener is required")
	}

	listenerProtocols := make(map[string]string, len(listeners))
	ports := map[int]struct{}{}
	for i := range listeners {
		listener := &listeners[i]
		if listener.Name == "" {
			return errors.New("listener name is required")
		}
		if _, ok := listenerProtocols[listener.Name]; ok {
			return fmt.Errorf("listener %q is duplicated", listener.Name)
		}
		if listener.Protocol != string(resource.ProtocolHTTP) && listener.Protocol != string(resource.ProtocolHTTPS) {
			return errors.New("listener protocol must be HTTP or HTTPS")
		}
		if listener.Port < 1 || listener.Port > 65535 {
			return errors.New("listener port must be between 1 and 65535")
		}
		if _, ok := ports[listener.Port]; ok {
			return errors.New("listener ports cannot be duplicated")
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
				return errors.New("only one catch-all host binding is allowed")
			}
		} else if !validHostname(binding.Hostname) {
			return errors.New("gateway hostname is invalid")
		}

		if len(binding.ListenerRefs) == 0 {
			return errors.New("host binding listenerRefs is required")
		}
		hasHTTPS := false
		for j := range binding.ListenerRefs {
			listenerRef := binding.ListenerRefs[j]
			if listenerRef == "" {
				return errors.New("host binding listenerRef cannot be empty")
			}
			protocol, ok := listenerProtocols[listenerRef]
			if !ok {
				return fmt.Errorf("host binding references unknown listener %q", listenerRef)
			}
			if protocol == string(resource.ProtocolHTTPS) {
				hasHTTPS = true
			}
		}

		certificateRef := ""
		if binding.TLS != nil {
			certificateRef = binding.TLS.CertificateRef
		}
		if hasHTTPS && certificateRef == "" {
			return errors.New("HTTPS host binding certificateRef is required")
		}
		if !hasHTTPS && certificateRef != "" {
			return errors.New("HTTP host binding cannot set certificateRef")
		}
		if hasHTTPS {
			for _, listenerRef := range binding.ListenerRefs {
				if listenerProtocols[listenerRef] == string(resource.ProtocolHTTPS) {
					httpsListenersWithTLS[listenerRef] = struct{}{}
				}
			}
		}
	}

	for name, protocol := range listenerProtocols {
		if protocol != string(resource.ProtocolHTTPS) {
			continue
		}
		if _, ok := httpsListenersWithTLS[name]; !ok {
			return fmt.Errorf("HTTPS listener %q must be referenced by a TLS host binding", name)
		}
	}
	return nil
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

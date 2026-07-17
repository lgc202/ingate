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
		return errors.New("网关版本不能为空")
	}
	return r.GatewayConfig.Validate()
}

// Validate 校验 Gateway 配置请求
func (r *GatewayConfig) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return errors.New("gateway name is required")
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

	listenerNames := make(map[string]struct{}, len(listeners))
	ports := map[int]struct{}{}
	for i := range listeners {
		listener := &listeners[i]
		if listener.Name == "" {
			return errors.New("listener name is required")
		}
		if _, ok := listenerNames[listener.Name]; ok {
			return fmt.Errorf("listener %q is duplicated", listener.Name)
		}
		if listener.Protocol != string(resource.ProtocolHTTP) {
			return errors.New("listener protocol must be HTTP")
		}
		if listener.Port < 1 || listener.Port > 65535 {
			return errors.New("listener port must be between 1 and 65535")
		}
		if _, ok := ports[listener.Port]; ok {
			return errors.New("listener ports cannot be duplicated")
		}
		listenerNames[listener.Name] = struct{}{}
		ports[listener.Port] = struct{}{}
	}

	catchAllCount := 0
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
		for j := range binding.ListenerRefs {
			listenerRef := binding.ListenerRefs[j]
			if listenerRef == "" {
				return errors.New("host binding listenerRef cannot be empty")
			}
			if _, ok := listenerNames[listenerRef]; !ok {
				return fmt.Errorf("host binding references unknown listener %q", listenerRef)
			}
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

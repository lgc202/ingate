package dto

import "testing"

func TestGatewayConfigValidateAcceptsOnlyHTTP(t *testing.T) {
	config := GatewayConfig{
		Name: "测试网关",
		Listeners: []GatewayListener{
			{Name: "public", Protocol: "HTTP", Port: 8080},
		},
		HostBindings: []GatewayHostBinding{
			{ListenerRefs: []string{"public"}},
		},
	}
	if err := config.Validate(); err != nil {
		t.Errorf("GatewayConfig.Validate(HTTP listener) error = %v, want nil", err)
	}

	config.Listeners[0].Protocol = "HTTPS"
	if err := config.Validate(); err == nil {
		t.Error("GatewayConfig.Validate(HTTPS listener) error = nil, want non-nil")
	}
}

package app

import (
	"strings"
	"testing"

	"github.com/lgc202/go-kit/logx"
	"github.com/lgc202/ingate/internal/pkg/appconfig"
)

func TestConfigValidate_XDSListenAddress(t *testing.T) {
	tests := []struct {
		name      string
		address   string
		wantError bool
	}{
		{name: "IPv4 loopback", address: "127.0.0.1:18000"},
		{name: "IPv4 loopback subnet", address: "127.10.20.30:18000"},
		{name: "IPv6 loopback", address: "[::1]:18000"},
		{name: "hostname is not resolved", address: "localhost:18000", wantError: true},
		{name: "IPv4 wildcard", address: "0.0.0.0:18000", wantError: true},
		{name: "IPv6 wildcard", address: "[::]:18000", wantError: true},
		{name: "private address", address: "192.168.1.10:18000", wantError: true},
		{name: "missing port", address: "127.0.0.1", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validConfig()
			config.Server.XDSListenAddress = tt.address

			err := config.Validate()
			if gotError := err != nil; gotError != tt.wantError {
				t.Errorf("Config.Validate() with xDS address %q error = %v, want error presence = %t", tt.address, err, tt.wantError)
			}
			if tt.wantError && err != nil && (!strings.Contains(err.Error(), "sensitive configuration") || !strings.Contains(err.Error(), "mTLS")) {
				t.Errorf("Config.Validate() with xDS address %q error = %q, want sensitive configuration and mTLS guidance", tt.address, err)
			}
		})
	}
}

func validConfig() Config {
	return Config{
		Server: ServerConfig{
			XDSListenAddress:    "127.0.0.1:18000",
			HealthListenAddress: "127.0.0.1:18080",
		},
		Logging: appconfig.Logging{
			Format: logx.FormatJSON,
			Level:  logx.LevelInfo,
			Stdout: true,
		},
	}
}

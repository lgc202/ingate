package xredis_test

import (
	"strings"
	"testing"

	"github.com/lgc202/ingate/pkg/xredis"
)

func TestNewClientRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  xredis.Config
		wantErr string
	}{
		{
			name:    "missing address",
			config:  xredis.Config{},
			wantErr: "redis address is required",
		},
		{
			name: "unsupported mode",
			config: xredis.Config{
				Address: "127.0.0.1:6379",
				Mode:    xredis.Mode("Proxy"),
			},
			wantErr: "unsupported redis mode",
		},
		{
			name: "sentinel master required",
			config: xredis.Config{
				Address: "127.0.0.1:26379",
				Mode:    xredis.ModeSentinel,
			},
			wantErr: "redis sentinel master is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := xredis.NewClient(tt.config)
			if err == nil {
				_ = client.Close()
				t.Fatal("NewClient() error = nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("NewClient() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestManagerReusesClientForSameConfig(t *testing.T) {
	manager := xredis.NewManager()
	defer manager.Close()

	config := xredis.Config{
		ID:      "redis-main",
		Address: "127.0.0.1:6379",
	}
	first, err := manager.Client(config)
	if err != nil {
		t.Fatalf("Client() error = %v", err)
	}
	second, err := manager.Client(config)
	if err != nil {
		t.Fatalf("Client() error = %v", err)
	}
	if first != second {
		t.Fatal("Client() returned different clients for the same config")
	}
}

func TestManagerReplacesClientWhenConfigChanges(t *testing.T) {
	manager := xredis.NewManager()
	defer manager.Close()

	first, err := manager.Client(xredis.Config{
		ID:      "redis-main",
		Address: "127.0.0.1:6379",
	})
	if err != nil {
		t.Fatalf("Client() error = %v", err)
	}
	second, err := manager.Client(xredis.Config{
		ID:      "redis-main",
		Address: "127.0.0.1:6380",
	})
	if err != nil {
		t.Fatalf("Client() error = %v", err)
	}
	if first == second {
		t.Fatal("Client() reused old client after config changed")
	}
}

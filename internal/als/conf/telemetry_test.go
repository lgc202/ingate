package conf

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
)

// TestValidateTelemetry 验证可选 Trace 出口的配置边界。
func TestValidateTelemetry(t *testing.T) {
	tests := []struct {
		name    string
		config  *Telemetry
		wantErr bool
	}{
		{
			name:   "unconfigured",
			config: &Telemetry{Environment: "test"},
		},
		{
			name:   "empty endpoint disables tracing",
			config: &Telemetry{Environment: "test", Tracing: &Telemetry_Tracing{}},
		},
		{
			name:   "blank endpoint disables tracing",
			config: &Telemetry{Environment: "test", Tracing: &Telemetry_Tracing{Endpoint: " \t"}},
		},
		{
			name:   "enabled",
			config: &Telemetry{Environment: "test", Tracing: newTracingConfig()},
		},
		{
			name:    "missing environment",
			config:  &Telemetry{Tracing: newTracingConfig()},
			wantErr: true,
		},
		{
			name: "batch larger than queue",
			config: &Telemetry{Environment: "test", Tracing: &Telemetry_Tracing{
				Endpoint:        "collector:4317",
				SampleRatio:     0.1,
				MaxQueueSize:    1,
				ExportBatchSize: 2,
				BatchTimeout:    durationpb.New(time.Second),
				ExportTimeout:   durationpb.New(time.Second),
			}},
			wantErr: true,
		},
		{
			name: "TLS with insecure transport",
			config: &Telemetry{Environment: "test", Tracing: &Telemetry_Tracing{
				Endpoint:        "collector:4317",
				Insecure:        true,
				SampleRatio:     0.1,
				MaxQueueSize:    10,
				ExportBatchSize: 5,
				BatchTimeout:    durationpb.New(time.Second),
				ExportTimeout:   durationpb.New(time.Second),
				Tls:             &Telemetry_Tracing_TLS{CaFile: "ca.pem"},
			}},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateTelemetry(test.config)
			if got := err != nil; got != test.wantErr {
				t.Errorf("validateTelemetry(%s) error = %v, want error presence = %t", test.name, err, test.wantErr)
			}
		})
	}
}

func newTracingConfig() *Telemetry_Tracing {
	return &Telemetry_Tracing{
		Endpoint:        "collector:4317",
		Insecure:        true,
		SampleRatio:     0.1,
		MaxQueueSize:    2048,
		ExportBatchSize: 256,
		BatchTimeout:    durationpb.New(2 * time.Second),
		ExportTimeout:   durationpb.New(5 * time.Second),
	}
}

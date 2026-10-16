package validation

import (
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateBackend(t *testing.T) {
	base := gateway.Backend{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "backend-1"},
		Spec: gateway.BackendSpec{
			Endpoints: []gateway.Endpoint{{Address: "api.example.com", Port: 8080, Weight: 1}},
		},
	}

	tests := []struct {
		name       string
		update     func(*gateway.Backend)
		wantFields []string
	}{
		{name: "valid"},
		{
			name:       "missing namespace",
			update:     func(b *gateway.Backend) { b.Namespace = "" },
			wantFields: []string{"metadata.namespace"},
		},
		{
			name:       "invalid namespace",
			update:     func(b *gateway.Backend) { b.Namespace = "Team.A" },
			wantFields: []string{"metadata.namespace"},
		},
		{
			name:       "empty endpoints",
			update:     func(u *gateway.Backend) { u.Spec.Endpoints = nil },
			wantFields: []string{"spec.endpoints"},
		},
		{
			name:       "invalid address",
			update:     func(u *gateway.Backend) { u.Spec.Endpoints[0].Address = "bad host" },
			wantFields: []string{"spec.endpoints[0].address"},
		},
		{
			name:       "invalid port and weight",
			update:     func(u *gateway.Backend) { u.Spec.Endpoints[0].Port = 0; u.Spec.Endpoints[0].Weight = 0 },
			wantFields: []string{"spec.endpoints[0].port", "spec.endpoints[0].weight"},
		},
		{
			name: "same address and different port",
			update: func(u *gateway.Backend) {
				u.Spec.Endpoints = append(u.Spec.Endpoints, gateway.Endpoint{
					Address: "api.example.com",
					Port:    8081,
					Weight:  1,
				})
			},
		},
		{
			name: "duplicate address and port",
			update: func(u *gateway.Backend) {
				u.Spec.Endpoints = append(u.Spec.Endpoints, gateway.Endpoint{
					Address: "api.example.com",
					Port:    8080,
					Weight:  2,
				})
			},
			wantFields: []string{"spec.endpoints[1]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := *base.DeepCopy()
			if tt.update != nil {
				tt.update(&candidate)
			}

			errs := ValidateBackend(&candidate)
			var gotFields []string
			for _, err := range errs {
				gotFields = append(gotFields, err.Field)
			}

			if !slices.Equal(gotFields, tt.wantFields) {
				t.Errorf("ValidateBackend(%q) fields = %v, want %v", tt.name, gotFields, tt.wantFields)
			}
		})
	}
}

func TestValidateBackendTLS(t *testing.T) {
	for _, tt := range []struct {
		name    string
		tls     *gateway.BackendTLSSettings
		wantErr bool
	}{
		{name: "omitted"},
		{
			name: "fixed DNS",
			tls:  &gateway.BackendTLSSettings{SNI: "service.example.com", WellKnownCACertificates: "System"},
		},
		{name: "empty object", tls: &gateway.BackendTLSSettings{}, wantErr: true},
		{
			name:    "missing CA",
			tls:     &gateway.BackendTLSSettings{SNI: "service.example.com"},
			wantErr: true,
		},
		{
			name:    "private CA",
			tls:     &gateway.BackendTLSSettings{SNI: "service.example.com", WellKnownCACertificates: "Private"},
			wantErr: true,
		},
		{
			name:    "IP identity",
			tls:     &gateway.BackendTLSSettings{SNI: "192.0.2.1", WellKnownCACertificates: "System"},
			wantErr: true,
		},
		{
			name:    "IPv6 identity",
			tls:     &gateway.BackendTLSSettings{SNI: "2001:db8::1", WellKnownCACertificates: "System"},
			wantErr: true,
		},
		{
			name:    "wildcard",
			tls:     &gateway.BackendTLSSettings{SNI: "*.example.com", WellKnownCACertificates: "System"},
			wantErr: true,
		},
		{
			name:    "authority",
			tls:     &gateway.BackendTLSSettings{SNI: "service.example.com:443", WellKnownCACertificates: "System"},
			wantErr: true,
		},
		{
			name:    "URL",
			tls:     &gateway.BackendTLSSettings{SNI: "https://service.example.com", WellKnownCACertificates: "System"},
			wantErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			backend := &gateway.Backend{
				ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "backend"},
				Spec: gateway.BackendSpec{
					Endpoints: []gateway.Endpoint{{Address: "2001:db8::1", Port: 8443, Weight: 1}},
					TLS:       tt.tls,
				},
			}

			errs := ValidateBackend(backend)

			if (len(errs) != 0) != tt.wantErr {
				t.Errorf("ValidateBackend(TLS=%+v) errors = %v, want error presence %t", tt.tls, errs, tt.wantErr)
			}
		})
	}
}

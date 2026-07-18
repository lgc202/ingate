package gateway

import (
	"context"
	"testing"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestStatusStrategyPreservesSpecAndManagedMetadata(t *testing.T) {
	oldGateway := &resource.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "gateway-1",
			Generation:  4,
			Labels:      map[string]string{"owner": "apiserver"},
			Annotations: map[string]string{"source": "spec"},
		},
		Spec: resource.GatewaySpec{DisplayName: "old name"},
	}
	updated := oldGateway.DeepCopy()
	updated.Generation = 99
	updated.Labels = map[string]string{"owner": "status-writer"}
	updated.Annotations = map[string]string{"source": "status"}
	updated.Spec.DisplayName = "changed through status"
	updated.Status.Conditions = []metav1.Condition{{
		Type:   "Accepted",
		Status: metav1.ConditionTrue,
		Reason: "Accepted",
	}}

	statusStrategy{}.PrepareForUpdate(context.Background(), updated, oldGateway)

	if !apiequality.Semantic.DeepEqual(updated.Spec, oldGateway.Spec) {
		t.Errorf("status PrepareForUpdate() spec = %#v, want %#v", updated.Spec, oldGateway.Spec)
	}
	if updated.Generation != oldGateway.Generation {
		t.Errorf("status PrepareForUpdate() generation = %d, want %d", updated.Generation, oldGateway.Generation)
	}
	if !apiequality.Semantic.DeepEqual(updated.Labels, oldGateway.Labels) {
		t.Errorf("status PrepareForUpdate() labels = %v, want %v", updated.Labels, oldGateway.Labels)
	}
	if !apiequality.Semantic.DeepEqual(updated.Annotations, oldGateway.Annotations) {
		t.Errorf("status PrepareForUpdate() annotations = %v, want %v", updated.Annotations, oldGateway.Annotations)
	}
	if len(updated.Status.Conditions) != 1 || updated.Status.Conditions[0].Type != "Accepted" {
		t.Errorf("status PrepareForUpdate() conditions = %v, want submitted status", updated.Status.Conditions)
	}
}

func TestValidateGatewayListenerCertificateRef(t *testing.T) {
	tests := []struct {
		name           string
		protocol       resource.Protocol
		certificateRef string
		wantField      string
	}{
		{
			name:     "HTTP listener",
			protocol: resource.ProtocolHTTP,
		},
		{
			name:           "HTTPS listener",
			protocol:       resource.ProtocolHTTPS,
			certificateRef: "certificate-id",
		},
		{
			name:      "HTTPS listener requires certificate",
			protocol:  resource.ProtocolHTTPS,
			wantField: "spec.listeners[0].certificateRef",
		},
		{
			name:           "HTTP listener rejects certificate",
			protocol:       resource.ProtocolHTTP,
			certificateRef: "certificate-id",
			wantField:      "spec.listeners[0].certificateRef",
		},
		{
			name:      "unsupported protocol",
			protocol:  resource.Protocol("TCP"),
			wantField: "spec.listeners[0].protocol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := &resource.Gateway{Spec: resource.GatewaySpec{
				DisplayName: "gateway",
				Listeners: []resource.Listener{{
					Name:           "public",
					Protocol:       tt.protocol,
					Port:           8080,
					CertificateRef: tt.certificateRef,
				}},
			}}

			errs := validateGateway(gateway)
			if tt.wantField == "" {
				if len(errs) != 0 {
					t.Errorf("validateGateway(%q) errors = %v, want none", tt.name, errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("validateGateway(%q) errors = %v, want one error for %q", tt.name, errs, tt.wantField)
			}
			if errs[0].Field != tt.wantField {
				t.Errorf("validateGateway(%q) error field = %q, want %q", tt.name, errs[0].Field, tt.wantField)
			}
		})
	}
}

func TestValidateGatewayHostBindings(t *testing.T) {
	tests := []struct {
		name      string
		bindings  []resource.HostBinding
		wantField string
	}{
		{
			name: "distinct exact hostnames",
			bindings: []resource.HostBinding{
				{Hostname: "api.example.com", ListenerRefs: []string{"public"}},
				{Hostname: "mcp.example.com", ListenerRefs: []string{"public"}},
			},
		},
		{
			name: "same exact hostname",
			bindings: []resource.HostBinding{
				{Hostname: "api.example.com", ListenerRefs: []string{"public"}},
				{Hostname: "api.example.com", ListenerRefs: []string{"public"}},
			},
			wantField: "spec.hostBindings[1].hostname",
		},
		{
			name: "wildcard overlaps exact hostname",
			bindings: []resource.HostBinding{
				{Hostname: "*.example.com", ListenerRefs: []string{"public"}},
				{Hostname: "api.example.com", ListenerRefs: []string{"public"}},
			},
			wantField: "spec.hostBindings[1].hostname",
		},
		{
			name: "catch all overlaps exact hostname",
			bindings: []resource.HostBinding{
				{Hostname: "", ListenerRefs: []string{"public"}},
				{Hostname: "api.example.com", ListenerRefs: []string{"public"}},
			},
			wantField: "spec.hostBindings[1].hostname",
		},
		{
			name: "same hostname on different listeners",
			bindings: []resource.HostBinding{
				{Hostname: "api.example.com", ListenerRefs: []string{"public"}},
				{Hostname: "api.example.com", ListenerRefs: []string{"admin"}},
			},
		},
		{
			name: "duplicate listener reference",
			bindings: []resource.HostBinding{
				{Hostname: "api.example.com", ListenerRefs: []string{"public", "public"}},
			},
			wantField: "spec.hostBindings[0].listenerRefs[1]",
		},
		{
			name: "literal catch all is not allowed",
			bindings: []resource.HostBinding{
				{Hostname: "*", ListenerRefs: []string{"public"}},
			},
			wantField: "spec.hostBindings[0].hostname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := &resource.Gateway{Spec: resource.GatewaySpec{
				DisplayName: "gateway",
				Listeners: []resource.Listener{
					{Name: "public", Protocol: resource.ProtocolHTTP, Port: 8080},
					{Name: "admin", Protocol: resource.ProtocolHTTP, Port: 8081},
				},
				HostBindings: tt.bindings,
			}}

			errs := validateGateway(gateway)
			if tt.wantField == "" {
				if len(errs) != 0 {
					t.Errorf("validateGateway(%q) errors = %v, want none", tt.name, errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("validateGateway(%q) errors = %v, want one error for %q", tt.name, errs, tt.wantField)
			}
			if errs[0].Field != tt.wantField {
				t.Errorf("validateGateway(%q) error field = %q, want %q", tt.name, errs[0].Field, tt.wantField)
			}
		})
	}
}

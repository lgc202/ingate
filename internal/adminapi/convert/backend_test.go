package convert

import (
	"testing"

	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestBackendProtocolRoundTrip(t *testing.T) {
	req := dto.CreateBackendRequest{
		Name:        "backend",
		Type:        "Static",
		Protocol:    gatewayv1alpha1.BackendProtocolHTTPS,
		DefaultPort: 8080,
		Static: &dto.StaticBackendSpec{
			Endpoints: []dto.BackendEndpoint{
				{Address: "127.0.0.1", Port: 8080, Weight: 100},
			},
		},
	}

	backend := BackendFromCreateRequest(req)
	if backend.Spec.Protocol != req.Protocol {
		t.Fatalf("expected backend protocol %q, got %q", req.Protocol, backend.Spec.Protocol)
	}

	resp := BackendToResponse(backend)
	if resp.Spec.Protocol != req.Protocol {
		t.Fatalf("expected response protocol %q, got %q", req.Protocol, resp.Spec.Protocol)
	}
}

func TestBackendToResponseNormalizesLegacyEmptyProtocol(t *testing.T) {
	backend := &gatewayv1alpha1.Backend{
		Spec: gatewayv1alpha1.BackendSpec{
			Type:        "Static",
			Protocol:    "",
			DefaultPort: 8080,
		},
	}

	resp := BackendToResponse(backend)
	if resp.Spec.Protocol != gatewayv1alpha1.BackendProtocolHTTP {
		t.Fatalf("expected normalized protocol %q, got %q", gatewayv1alpha1.BackendProtocolHTTP, resp.Spec.Protocol)
	}
}

func TestBackendListToResponseNormalizesLegacyEmptyProtocol(t *testing.T) {
	list := &gatewayv1alpha1.BackendList{
		Items: []gatewayv1alpha1.Backend{
			{
				Spec: gatewayv1alpha1.BackendSpec{
					Type:        "Static",
					Protocol:    "",
					DefaultPort: 8080,
				},
			},
			{
				Spec: gatewayv1alpha1.BackendSpec{
					Type:        "DNS",
					Protocol:    gatewayv1alpha1.BackendProtocolHTTPS,
					DefaultPort: 8081,
				},
			},
		},
	}

	resp := BackendListToResponse(list)
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	if resp.Items[0].Spec.Protocol != gatewayv1alpha1.BackendProtocolHTTP {
		t.Fatalf("expected first item protocol %q, got %q", gatewayv1alpha1.BackendProtocolHTTP, resp.Items[0].Spec.Protocol)
	}
	if resp.Items[1].Spec.Protocol != gatewayv1alpha1.BackendProtocolHTTPS {
		t.Fatalf("expected second item protocol %q, got %q", gatewayv1alpha1.BackendProtocolHTTPS, resp.Items[1].Spec.Protocol)
	}
}

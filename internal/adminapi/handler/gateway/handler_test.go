package gateway

import (
	"context"
	"testing"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestCreateGateway(t *testing.T) {
	client := clientfake.NewSimpleClientset()
	gateways := repository.NewGateway(client.GatewayV1alpha1().Gateways(), nil)
	handler := New(gatewayservice.New(gateways))
	request := &adminv1.CreateGatewayRequest{
		DisplayName: "Public Gateway",
		Listeners: []*adminv1.Listener{{
			Name:     "http",
			Protocol: adminv1.ListenerProtocol_LISTENER_PROTOCOL_HTTP,
			Port:     8080,
		}},
	}

	reply, err := handler.CreateGateway(context.Background(), request)
	if err != nil {
		t.Fatalf("CreateGateway(public-gateway) error = %v, want nil", err)
	}
	if name := reply.GetGateway().GetDisplayName(); name != "Public Gateway" {
		t.Errorf("CreateGateway(public-gateway) display name = %q, want Public Gateway", name)
	}
	listeners := reply.GetGateway().GetListeners()
	if len(listeners) != 1 || listeners[0].GetPort() != 8080 {
		t.Errorf("CreateGateway(public-gateway) listeners = %v, want one listener on port 8080", listeners)
	}

	second, err := handler.CreateGateway(context.Background(), request)
	if err != nil {
		t.Fatalf("CreateGateway(duplicate display name) error = %v, want nil", err)
	}
	if second.GetGateway().GetName() == reply.GetGateway().GetName() || second.GetGateway().GetDisplayName() != request.DisplayName {
		t.Errorf("CreateGateway(duplicate display name) = %v, want distinct ID and unchanged display name", second)
	}
}

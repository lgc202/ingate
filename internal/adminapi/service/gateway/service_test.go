package gateway

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	"github.com/lgc202/ingate/internal/adminapi/service/policytarget"
	accesscontrolpolicystore "github.com/lgc202/ingate/internal/adminapi/store/accesscontrolpolicy"
	certificatestore "github.com/lgc202/ingate/internal/adminapi/store/certificate"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	ratelimitpolicystore "github.com/lgc202/ingate/internal/adminapi/store/ratelimitpolicy"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGatewaySpecBuildsStandaloneListeners(t *testing.T) {
	listenerParams := []ListenerParams{
		{Protocol: resource.ProtocolHTTP, Port: 8080},
		{Protocol: resource.ProtocolHTTPS, Port: 8443, CertificateID: "certificate-1"},
	}
	listeners := []resource.Listener{
		{Name: "http", Protocol: resource.ProtocolHTTP, Port: 8080},
		{Name: "https", Protocol: resource.ProtocolHTTPS, Port: 8443, CertificateRef: "certificate-1"},
	}
	listenerRefs := []string{"http", "https"}
	tests := []struct {
		name      string
		hostnames []string
		bindings  []resource.HostBinding
	}{
		{
			name: "catch all",
		},
		{
			name:      "explicit hostnames",
			hostnames: []string{"api.example.com", "*.model.example.com"},
			bindings: []resource.HostBinding{
				{Hostname: "api.example.com", ListenerRefs: listenerRefs},
				{Hostname: "*.model.example.com", ListenerRefs: listenerRefs},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gatewaySpec(GatewayParams{
				Name:        "public gateway",
				Description: "public traffic",
				Listeners:   listenerParams,
				Hostnames:   tt.hostnames,
			}, true)
			want := resource.GatewaySpec{
				DisplayName:  "public gateway",
				Description:  "public traffic",
				Enabled:      true,
				Listeners:    listeners,
				HostBindings: tt.bindings,
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf("gatewaySpec(%v, %v) = %#v, want %#v", listenerParams, tt.hostnames, got, want)
			}
		})
	}
}

func TestServiceCreateValidatesGatewayListenerOwnership(t *testing.T) {
	tests := []struct {
		name              string
		existingProtocol  resource.Protocol
		existingPort      int
		existingHostnames []string
		existingEnabled   bool
		protocol          resource.Protocol
		port              int
		hostnames         []string
		wantErr           bool
	}{
		{
			name:             "two catch all gateways",
			existingProtocol: resource.ProtocolHTTP,
			existingPort:     8080,
			existingEnabled:  true,
			protocol:         resource.ProtocolHTTP,
			port:             8080,
			wantErr:          true,
		},
		{
			name:             "catch all owns exact hostname",
			existingProtocol: resource.ProtocolHTTP,
			existingPort:     8080,
			existingEnabled:  true,
			protocol:         resource.ProtocolHTTP,
			port:             8080,
			hostnames:        []string{"api.example.com"},
			wantErr:          true,
		},
		{
			name:              "exact hostname owns catch all",
			existingProtocol:  resource.ProtocolHTTP,
			existingPort:      8080,
			existingHostnames: []string{"api.example.com"},
			existingEnabled:   true,
			protocol:          resource.ProtocolHTTP,
			port:              8080,
			wantErr:           true,
		},
		{
			name:              "same exact hostname",
			existingProtocol:  resource.ProtocolHTTP,
			existingPort:      8080,
			existingHostnames: []string{"api.example.com"},
			existingEnabled:   true,
			protocol:          resource.ProtocolHTTP,
			port:              8080,
			hostnames:         []string{"api.example.com"},
			wantErr:           true,
		},
		{
			name:              "wildcard owns exact hostname",
			existingProtocol:  resource.ProtocolHTTPS,
			existingPort:      8443,
			existingHostnames: []string{"*.example.com"},
			existingEnabled:   true,
			protocol:          resource.ProtocolHTTPS,
			port:              8443,
			hostnames:         []string{"api.example.com"},
			wantErr:           true,
		},
		{
			name:              "nested wildcards",
			existingProtocol:  resource.ProtocolHTTPS,
			existingPort:      8443,
			existingHostnames: []string{"*.example.com"},
			existingEnabled:   true,
			protocol:          resource.ProtocolHTTPS,
			port:              8443,
			hostnames:         []string{"*.api.example.com"},
			wantErr:           true,
		},
		{
			name:              "different exact hostnames",
			existingProtocol:  resource.ProtocolHTTP,
			existingPort:      8080,
			existingHostnames: []string{"api.example.com"},
			existingEnabled:   true,
			protocol:          resource.ProtocolHTTP,
			port:              8080,
			hostnames:         []string{"mcp.example.com"},
		},
		{
			name:              "wildcard does not own apex",
			existingProtocol:  resource.ProtocolHTTPS,
			existingPort:      8443,
			existingHostnames: []string{"*.example.com"},
			existingEnabled:   true,
			protocol:          resource.ProtocolHTTPS,
			port:              8443,
			hostnames:         []string{"example.com"},
		},
		{
			name:              "same hostname on different ports",
			existingProtocol:  resource.ProtocolHTTP,
			existingPort:      8081,
			existingHostnames: []string{"api.example.com"},
			existingEnabled:   true,
			protocol:          resource.ProtocolHTTP,
			port:              8080,
			hostnames:         []string{"api.example.com"},
		},
		{
			name:              "same hostname on HTTP and HTTPS default ports",
			existingProtocol:  resource.ProtocolHTTP,
			existingPort:      8080,
			existingHostnames: []string{"api.example.com"},
			existingEnabled:   true,
			protocol:          resource.ProtocolHTTPS,
			port:              8443,
			hostnames:         []string{"api.example.com"},
		},
		{
			name:              "disabled gateway does not own hostname",
			existingProtocol:  resource.ProtocolHTTP,
			existingPort:      8080,
			existingHostnames: []string{"api.example.com"},
			existingEnabled:   false,
			protocol:          resource.ProtocolHTTP,
			port:              8080,
			hostnames:         []string{"api.example.com"},
		},
		{
			name:              "different protocols cannot share port",
			existingProtocol:  resource.ProtocolHTTP,
			existingPort:      8443,
			existingHostnames: []string{"api.example.com"},
			existingEnabled:   true,
			protocol:          resource.ProtocolHTTPS,
			port:              8443,
			hostnames:         []string{"mcp.example.com"},
			wantErr:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certificate := &resource.Certificate{ObjectMeta: metav1.ObjectMeta{Name: "certificate-1"}}
			existing := testGateway(
				"existing-gateway",
				"existing gateway",
				tt.existingEnabled,
				tt.existingProtocol,
				tt.existingPort,
				tt.existingHostnames,
				certificate.Name,
			)
			client := clientfake.NewSimpleClientset()
			if _, err := client.GatewayV1().Gateways().Create(context.Background(), existing, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Gateways.Create(%q) error = %v", existing.Name, err)
			}
			if _, err := client.GatewayV1().Certificates().Create(context.Background(), certificate, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Certificates.Create(%q) error = %v", certificate.Name, err)
			}
			service := newTestService(client)

			_, err := service.Create(context.Background(), CreateGatewayParams{GatewayParams: GatewayParams{
				Name:      "new gateway",
				Listeners: []ListenerParams{{Protocol: tt.protocol, Port: tt.port, CertificateID: listenerCertificateID(tt.protocol, certificate.Name)}},
				Hostnames: tt.hostnames,
			}})
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("Service.Create(%q) error = %v, want error presence = %t", tt.name, err, tt.wantErr)
			}
			if tt.wantErr {
				var userError *xerrors.UserError
				if !errors.As(err, &userError) {
					t.Errorf("Service.Create(%q) error = %T, want *xerrors.UserError", tt.name, err)
				}
			}
		})
	}
}

func TestServiceSetEnabledRejectsHostnameConflict(t *testing.T) {
	existing := testGateway(
		"existing-gateway",
		"existing gateway",
		true,
		resource.ProtocolHTTP,
		8080,
		[]string{"api.example.com"},
		"",
	)
	disabled := testGateway(
		"disabled-gateway",
		"disabled gateway",
		false,
		resource.ProtocolHTTP,
		8080,
		[]string{"api.example.com"},
		"",
	)
	client := clientfake.NewSimpleClientset()
	for _, gateway := range []*resource.Gateway{existing, disabled} {
		if _, err := client.GatewayV1().Gateways().Create(context.Background(), gateway, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Gateways.Create(%q) error = %v", gateway.Name, err)
		}
	}
	service := newTestService(client)

	err := service.SetEnabled(context.Background(), disabled.Name, true)
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Errorf("Service.SetEnabled(%q, true) error = %T, want *xerrors.UserError", disabled.Name, err)
	}
	stored, getErr := client.GatewayV1().Gateways().Get(context.Background(), disabled.Name, metav1.GetOptions{})
	if getErr != nil {
		t.Fatalf("Gateways.Get(%q) error = %v", disabled.Name, getErr)
	}
	if stored.Spec.Enabled {
		t.Errorf("Service.SetEnabled(%q, true) stored enabled = true, want false after rejected update", disabled.Name)
	}
}

func TestServiceSetEnabledAllowsDisablingGatewayWithMissingCertificate(t *testing.T) {
	gateway := testGateway(
		"gateway-with-missing-certificate",
		"gateway with missing certificate",
		true,
		resource.ProtocolHTTPS,
		8443,
		[]string{"api.example.com"},
		"missing-certificate",
	)
	client := clientfake.NewSimpleClientset()
	if _, err := client.GatewayV1().Gateways().Create(context.Background(), gateway, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Gateways.Create(%q) error = %v", gateway.Name, err)
	}
	service := newTestService(client)

	if err := service.SetEnabled(context.Background(), gateway.Name, false); err != nil {
		t.Fatalf("Service.SetEnabled(%q, false) error = %v", gateway.Name, err)
	}
	stored, err := client.GatewayV1().Gateways().Get(context.Background(), gateway.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Gateways.Get(%q) error = %v", gateway.Name, err)
	}
	if stored.Spec.Enabled {
		t.Errorf("Service.SetEnabled(%q, false) stored enabled = true, want false", gateway.Name)
	}
}

func TestServiceCreateSerializesConflictValidation(t *testing.T) {
	client := clientfake.NewSimpleClientset()
	service := newTestService(client)

	const attempts = 16
	start := make(chan struct{})
	errorsByAttempt := make(chan error, attempts)
	var waitGroup sync.WaitGroup
	for attempt := range attempts {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			_, err := service.Create(context.Background(), CreateGatewayParams{GatewayParams: GatewayParams{
				Name:      fmt.Sprintf("gateway-%d", attempt),
				Listeners: []ListenerParams{{Protocol: resource.ProtocolHTTP, Port: 8080}},
			}})
			errorsByAttempt <- err
		}()
	}

	close(start)
	waitGroup.Wait()
	close(errorsByAttempt)

	successes := 0
	for err := range errorsByAttempt {
		if err == nil {
			successes++
			continue
		}
		var userError *xerrors.UserError
		if !errors.As(err, &userError) {
			t.Errorf("Service.Create() error = %T, want *xerrors.UserError", err)
		}
	}
	if successes != 1 {
		t.Errorf("concurrent Service.Create() successes = %d, want 1", successes)
	}

	stored, err := client.GatewayV1().Gateways().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Gateways.List() error = %v", err)
	}
	if len(stored.Items) != 1 {
		t.Errorf("stored gateways = %d, want 1", len(stored.Items))
	}
}

func TestServiceUpdateRejectsHostnameConflict(t *testing.T) {
	existing := testGateway(
		"existing-gateway",
		"existing gateway",
		true,
		resource.ProtocolHTTP,
		8080,
		[]string{"api.example.com"},
		"",
	)
	target := testGateway(
		"target-gateway",
		"target gateway",
		true,
		resource.ProtocolHTTP,
		8080,
		[]string{"mcp.example.com"},
		"",
	)
	target.ResourceVersion = "1"
	client := clientfake.NewSimpleClientset()
	for _, gateway := range []*resource.Gateway{existing, target} {
		if _, err := client.GatewayV1().Gateways().Create(context.Background(), gateway, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Gateways.Create(%q) error = %v", gateway.Name, err)
		}
	}
	service := newTestService(client)

	err := service.Update(context.Background(), target.Name, UpdateGatewayParams{
		Version: target.ResourceVersion,
		GatewayParams: GatewayParams{
			Name:      target.Spec.DisplayName,
			Listeners: []ListenerParams{{Protocol: resource.ProtocolHTTP, Port: 8080}},
			Hostnames: []string{"api.example.com"},
		},
	})
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Errorf("Service.Update(%q) error = %T, want *xerrors.UserError", target.Name, err)
	}
}

func TestServiceDeleteRejectsGatewayUsedByPolicy(t *testing.T) {
	gateway := testGateway(
		"gateway-1",
		"生产网关",
		true,
		resource.ProtocolHTTP,
		8080,
		nil,
		"",
	)
	policy := &resource.RateLimitPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "rate-limit-1"},
		Spec: resource.RateLimitPolicySpec{
			DisplayName: "公共接口限流",
			TargetRefs:  []resource.PolicyTargetRef{{Kind: resource.KindGateway, Name: gateway.Name}},
		},
	}
	client := clientfake.NewSimpleClientset()
	if _, err := client.GatewayV1().Gateways().Create(context.Background(), gateway, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Gateways.Create(%q) error = %v", gateway.Name, err)
	}
	if _, err := client.GatewayV1().RateLimitPolicies().Create(context.Background(), policy, metav1.CreateOptions{}); err != nil {
		t.Fatalf("RateLimitPolicies.Create(%q) error = %v", policy.Name, err)
	}
	service := newTestService(client)

	err := service.Delete(context.Background(), gateway.Name)
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Errorf("Service.Delete(%q) error = %T, want *xerrors.UserError", gateway.Name, err)
	}
	if _, err := client.GatewayV1().Gateways().Get(context.Background(), gateway.Name, metav1.GetOptions{}); err != nil {
		t.Errorf("Service.Delete(%q) removed referenced gateway: %v", gateway.Name, err)
	}
}

func newTestService(client *clientfake.Clientset) *Service {
	policyUsage := policytarget.NewUsageFinder(
		ratelimitpolicystore.New(client),
		accesscontrolpolicystore.New(client),
	)
	return New(
		gatewaystore.New(client),
		routestore.New(client),
		certificatestore.New(client),
		policyUsage,
	)
}

func testGateway(
	id string,
	displayName string,
	enabled bool,
	protocol resource.Protocol,
	port int,
	hostnames []string,
	certificateRef string,
) *resource.Gateway {
	listenerName := listenerName(protocol)
	gateway := &resource.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: id},
		Spec: resource.GatewaySpec{
			DisplayName: displayName,
			Enabled:     enabled,
			Listeners: []resource.Listener{{
				Name:           listenerName,
				Protocol:       protocol,
				Port:           port,
				CertificateRef: listenerCertificateID(protocol, certificateRef),
			}},
		},
	}
	for _, hostname := range hostnames {
		gateway.Spec.HostBindings = append(gateway.Spec.HostBindings, resource.HostBinding{
			Hostname:     hostname,
			ListenerRefs: []string{listenerName},
		})
	}
	return gateway
}

func listenerCertificateID(protocol resource.Protocol, id string) string {
	if protocol == resource.ProtocolHTTPS {
		return id
	}
	return ""
}

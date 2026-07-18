package certificate

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	certificatestore "github.com/lgc202/ingate/internal/adminapi/store/certificate"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestServiceDeleteRejectsReferencedCertificate(t *testing.T) {
	certificate := &resource.Certificate{ObjectMeta: metav1.ObjectMeta{Name: "certificate-1"}}
	gateway := &resource.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gateway-1"},
		Spec: resource.GatewaySpec{
			DisplayName: "public gateway",
			Listeners: []resource.Listener{
				{Protocol: resource.ProtocolHTTPS, Port: 8443, CertificateRef: certificate.Name},
			},
		},
	}
	client := clientfake.NewSimpleClientset()
	if _, err := client.GatewayV1().Certificates().Create(context.Background(), certificate, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Certificates.Create(certificate-1) error: %v", err)
	}
	if _, err := client.GatewayV1().Gateways().Create(context.Background(), gateway, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Gateways.Create(gateway-1) error: %v", err)
	}
	service := New(certificatestore.New(client), gatewaystore.New(client))

	err := service.Delete(context.Background(), certificate.Name)
	if err == nil {
		t.Fatal("Service.Delete(certificate-1) error = nil, want referenced certificate error")
	}
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Errorf("Service.Delete(certificate-1) error = %T, want *xerrors.UserError", err)
	}
}

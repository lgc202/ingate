package certificate

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateCertificate(t *testing.T) {
	certificatePEM, privateKeyPEM := newTestKeyPair(t, "gateway.example.com")
	_, otherPrivateKeyPEM := newTestKeyPair(t, "other.example.com")

	tests := []struct {
		name        string
		certificate *resource.Certificate
		wantFields  []string
	}{
		{
			name: "valid key pair",
			certificate: &resource.Certificate{Spec: resource.CertificateSpec{
				DisplayName:    "production certificate",
				CertificatePEM: certificatePEM,
				PrivateKeyPEM:  privateKeyPEM,
			}},
		},
		{
			name:        "required fields",
			certificate: &resource.Certificate{},
			wantFields: []string{
				"spec.displayName",
				"spec.certificatePEM",
				"spec.privateKeyPEM",
			},
		},
		{
			name: "mismatched private key",
			certificate: &resource.Certificate{Spec: resource.CertificateSpec{
				DisplayName:    "production certificate",
				CertificatePEM: certificatePEM,
				PrivateKeyPEM:  otherPrivateKeyPEM,
			}},
			wantFields: []string{"spec.certificatePEM"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateCertificate(tt.certificate)
			if len(errs) != len(tt.wantFields) {
				t.Fatalf("validateCertificate(%q) errors = %v, want fields %v", tt.name, errs, tt.wantFields)
			}
			for i, wantField := range tt.wantFields {
				if errs[i].Field != wantField {
					t.Errorf("validateCertificate(%q) error[%d].Field = %q, want %q", tt.name, i, errs[i].Field, wantField)
				}
			}
		})
	}
}

func newTestKeyPair(t *testing.T, commonName string) (string, string) {
	t.Helper()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v, want nil", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate(%q) error = %v, want nil", commonName, err)
	}
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey(%q) error = %v, want nil", commonName, err)
	}

	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
	return string(certificatePEM), string(privateKeyPEM)
}

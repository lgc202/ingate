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
)

func TestParseKeyPair(t *testing.T) {
	certificatePEM, privateKeyPEM := newTestKeyPair(t, "gateway.example.com")

	certificate, err := ParseKeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		t.Fatalf("ParseKeyPair(valid key pair) error = %v, want nil", err)
	}
	if certificate.Subject.CommonName != "gateway.example.com" {
		t.Errorf("ParseKeyPair(valid key pair).Subject.CommonName = %q, want %q", certificate.Subject.CommonName, "gateway.example.com")
	}
}

func TestParseKeyPairRejectsMismatchedPrivateKey(t *testing.T) {
	certificatePEM, _ := newTestKeyPair(t, "gateway.example.com")
	_, privateKeyPEM := newTestKeyPair(t, "other.example.com")

	if _, err := ParseKeyPair(certificatePEM, privateKeyPEM); err == nil {
		t.Error("ParseKeyPair(mismatched key pair) error = nil, want non-nil")
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

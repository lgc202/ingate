package certificate_test

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/lgc202/ingate/internal/pkg/certificate"
)

// TestParseKeyPair 验证叶子证书、完整证书链和可用于签名的私钥一并返回。
func TestParseKeyPair(t *testing.T) {
	certPEM, keyPEM := testKeyPair(t)
	otherCertPEM, _ := testKeyPair(t)
	block, _ := pem.Decode([]byte(certPEM))
	tests := []struct {
		name  string
		cert  string
		count int
	}{
		{name: "leaf", cert: certPEM, count: 1},
		{name: "chain", cert: certPEM + otherCertPEM, count: 2},
		{name: "whitespace", cert: " \n" + certPEM + "\t\n", count: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pair, err := certificate.ParseKeyPair(tt.cert, keyPEM)
			if err != nil {
				t.Fatalf("ParseKeyPair(%s) error = %v", tt.name, err)
			}
			if pair.Leaf == nil {
				t.Fatal("ParseKeyPair() Leaf = nil, want parsed leaf certificate")
			}
			if !bytes.Equal(pair.Leaf.Raw, block.Bytes) {
				t.Error("ParseKeyPair() leaf differs from the first PEM certificate")
			}
			if got := len(pair.Certificate); got != tt.count {
				t.Errorf("ParseKeyPair(%s) chain length = %d, want %d", tt.name, got, tt.count)
			}
			signer, ok := pair.PrivateKey.(crypto.Signer)
			if !ok {
				t.Fatalf("ParseKeyPair() private key type = %T, want crypto.Signer", pair.PrivateKey)
			}
			publicKey, err := x509.MarshalPKIXPublicKey(signer.Public())
			if err != nil {
				t.Fatalf("encode parsed public key: %v", err)
			}
			if !bytes.Equal(publicKey, pair.Leaf.RawSubjectPublicKeyInfo) {
				t.Error("ParseKeyPair() private key does not match the leaf public key")
			}
		})
	}
}

// TestParseKeyPairRejectsInvalidPEM 验证复用 TLS 解析结果不会放宽严格 PEM 校验。
func TestParseKeyPairRejectsInvalidPEM(t *testing.T) {
	certPEM, keyPEM := testKeyPair(t)
	_, otherKeyPEM := testKeyPair(t)
	certBlock, _ := pem.Decode([]byte(certPEM))
	keyBlock, _ := pem.Decode([]byte(keyPEM))
	certBlock.Headers = map[string]string{"Comment": "not allowed"}
	keyBlock.Headers = map[string]string{"Comment": "not allowed"}
	invalidCertPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: []byte("invalid DER"),
	}))

	tests := []struct {
		name string
		cert string
		key  string
	}{
		{name: "empty_certificate", key: keyPEM},
		{name: "empty_key", cert: certPEM},
		{name: "mismatched_key", cert: certPEM, key: otherKeyPEM},
		{name: "certificate_prefix", cert: "garbage\n" + certPEM, key: keyPEM},
		{name: "certificate_suffix", cert: certPEM + "garbage", key: keyPEM},
		{name: "key_prefix", cert: certPEM, key: "garbage\n" + keyPEM},
		{name: "key_suffix", cert: certPEM, key: keyPEM + "garbage"},
		{name: "multiple_keys", cert: certPEM, key: keyPEM + keyPEM},
		{name: "key_in_chain", cert: certPEM + keyPEM, key: keyPEM},
		{name: "invalid_chain_certificate", cert: certPEM + invalidCertPEM, key: keyPEM},
		{name: "certificate_headers", cert: string(pem.EncodeToMemory(certBlock)), key: keyPEM},
		{name: "key_headers", cert: certPEM, key: string(pem.EncodeToMemory(keyBlock))},
		{
			name: "oversized_certificate",
			cert: certPEM + strings.Repeat(" ", certificate.MaxCertificatePEMBytes),
			key:  keyPEM,
		},
		{
			name: "oversized_key",
			cert: certPEM,
			key:  keyPEM + strings.Repeat(" ", certificate.MaxPrivateKeyPEMBytes),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := certificate.ParseKeyPair(tt.cert, tt.key); err == nil {
				t.Errorf("ParseKeyPair(%s) error = nil, want invalid PEM or key-pair error", tt.name)
			}
		})
	}
}

func testKeyPair(t *testing.T) (string, string) {
	t.Helper()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:     time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatalf("create test certificate: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("encode test key: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})),
		string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
}

// Package certificate 提供 TLS 证书与私钥的解析能力
package certificate

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
)

// ParseKeyPair 校验证书链和私钥的 PEM 格式及配对关系，并返回叶子证书
func ParseKeyPair(certificatePEM, privateKeyPEM string) (*x509.Certificate, error) {
	keyPair, err := tls.X509KeyPair([]byte(certificatePEM), []byte(privateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("parse TLS certificate and private key: %w", err)
	}

	leaf, err := x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse leaf certificate: %w", err)
	}
	return leaf, nil
}

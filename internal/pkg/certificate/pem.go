// Package certificate 提供 TLS 证书与私钥的规范化和解析能力。
package certificate

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

const (
	// MaxCertificatePEMBytes 限制证书链的持久化大小。
	MaxCertificatePEMBytes = 256 << 10
	// MaxPrivateKeyPEMBytes 限制私钥的持久化大小。
	MaxPrivateKeyPEMBytes = 64 << 10
)

// NormalizePEM 规范化一个 PEM 文档的首尾空白和结尾换行。
func NormalizePEM(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return value + "\n"
}

// ParseLeafCertificate 解析证书链中的第一张证书。
func ParseLeafCertificate(certificatePEM string) (*x509.Certificate, error) {
	return parseCertificatePEM(certificatePEM)
}

// ParseKeyPair 校验证书链和私钥的 PEM 格式及配对关系，并返回叶子证书。
func ParseKeyPair(certificatePEM, privateKeyPEM string) (*x509.Certificate, error) {
	leaf, err := parseCertificatePEM(certificatePEM)
	if err != nil {
		return nil, err
	}
	if err := validatePrivateKeyPEM(privateKeyPEM); err != nil {
		return nil, err
	}

	if _, err := tls.X509KeyPair([]byte(certificatePEM), []byte(privateKeyPEM)); err != nil {
		return nil, fmt.Errorf("parse TLS certificate and private key: %w", err)
	}

	return leaf, nil
}

func parseCertificatePEM(value string) (*x509.Certificate, error) {
	if len(value) > MaxCertificatePEMBytes {
		return nil, fmt.Errorf("certificate PEM exceeds %d bytes", MaxCertificatePEMBytes)
	}
	blocks, err := parsePEMDocument(value)
	if err != nil {
		return nil, fmt.Errorf("decode certificate PEM: %w", err)
	}
	var leaf *x509.Certificate
	for index, block := range blocks {
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			return nil, fmt.Errorf("certificate PEM block %d must contain a certificate", index+1)
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse certificate PEM block %d: %w", index+1, err)
		}
		if index == 0 {
			leaf = certificate
		}
	}
	return leaf, nil
}

func validatePrivateKeyPEM(value string) error {
	if len(value) > MaxPrivateKeyPEMBytes {
		return fmt.Errorf("private key PEM exceeds %d bytes", MaxPrivateKeyPEMBytes)
	}
	blocks, err := parsePEMDocument(value)
	if err != nil {
		return fmt.Errorf("decode private key PEM: %w", err)
	}
	if len(blocks) != 1 ||
		!strings.HasSuffix(blocks[0].Type, "PRIVATE KEY") ||
		len(blocks[0].Headers) != 0 {
		return errors.New("private key PEM must contain exactly one unencrypted private key")
	}
	return nil
}

func parsePEMDocument(value string) ([]*pem.Block, error) {
	remaining := []byte(value)
	blocks := make([]*pem.Block, 0, 1)
	for {
		remaining = bytes.TrimSpace(remaining)
		if len(remaining) == 0 {
			break
		}
		if !bytes.HasPrefix(remaining, []byte("-----BEGIN ")) {
			return nil, errors.New("unexpected content outside PEM block")
		}
		block, rest := pem.Decode(remaining)
		if block == nil {
			return nil, errors.New("invalid PEM block")
		}
		blocks = append(blocks, block)
		remaining = rest
	}
	if len(blocks) == 0 {
		return nil, errors.New("PEM document is empty")
	}
	return blocks, nil
}

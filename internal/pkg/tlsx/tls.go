// Package tlsx 提供从证书文件构造 TLS 客户端和服务端配置的能力
package tlsx

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

// ClientConfig 定义服务端校验和可选的客户端证书
type ClientConfig struct {
	Enabled         bool
	CAFile          string
	CertificateFile string
	PrivateKeyFile  string
	ServerName      string
}

// ServerConfig 定义服务端证书和可选的客户端证书校验
type ServerConfig struct {
	Enabled         bool
	CertificateFile string
	PrivateKeyFile  string
	ClientCAFile    string
}

// NewClient 创建最低使用 TLS 1.2 的客户端配置
// CAFile 为空时使用操作系统根证书，配置证书和私钥时启用 mTLS
func NewClient(config ClientConfig) (*tls.Config, error) {
	if !config.Enabled {
		return nil, nil
	}
	if (config.CertificateFile == "") != (config.PrivateKeyFile == "") {
		return nil, errors.New("TLS client certificate and private key must be configured together")
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: config.ServerName,
	}
	if config.CAFile != "" {
		roots, err := loadCertPool(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("load TLS server CA: %w", err)
		}
		tlsConfig.RootCAs = roots
	}
	if config.CertificateFile != "" {
		certificate, err := tls.LoadX509KeyPair(config.CertificateFile, config.PrivateKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load TLS client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	return tlsConfig, nil
}

// NewServer 创建最低使用 TLS 1.2 的服务端配置
// ClientCAFile 非空时要求客户端提供由该 CA 签发的证书
func NewServer(config ServerConfig) (*tls.Config, error) {
	if !config.Enabled {
		return nil, nil
	}
	if config.CertificateFile == "" || config.PrivateKeyFile == "" {
		return nil, errors.New("TLS server certificate and private key are required")
	}
	certificate, err := tls.LoadX509KeyPair(config.CertificateFile, config.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load TLS server certificate: %w", err)
	}
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
	}
	if config.ClientCAFile == "" {
		return tlsConfig, nil
	}

	clientCAs, err := loadCertPool(config.ClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("load TLS client CA: %w", err)
	}
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	tlsConfig.ClientCAs = clientCAs
	return tlsConfig, nil
}

func loadCertPool(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read CA certificate %q: %w", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("parse CA certificate %q", path)
	}
	return pool, nil
}

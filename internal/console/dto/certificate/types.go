// Package certificate 定义证书管理接口的请求和响应模型
package certificate

import consoledto "github.com/lgc202/ingate/internal/console/dto"

// CreateCertificateReq 是创建 Certificate 的请求体
type CreateCertificateReq struct {
	CertificateConfig
}

// UpdateCertificateReq 是更新 Certificate 的请求体
type UpdateCertificateReq struct {
	Version string `json:"version"`
	CertificateConfig
}

// CertificateConfig 是控制台读写 Certificate 时复用的核心配置
type CertificateConfig struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	CertificatePEM string `json:"certificatePEM,omitempty"`
	PrivateKeyPEM  string `json:"privateKeyPEM,omitempty"`
}

// Certificate 是 Console API 返回的证书对象
type Certificate struct {
	ID             string                    `json:"id"`
	Version        string                    `json:"version,omitempty"`
	Status         consoledto.ResourceStatus `json:"status"`
	Name           string                    `json:"name"`
	Description    string                    `json:"description"`
	CertificatePEM string                    `json:"certificatePEM,omitempty"`
	DNSNames       []string                  `json:"dnsNames"`
	NotBefore      string                    `json:"notBefore"`
	NotAfter       string                    `json:"notAfter"`
	CreatedAt      string                    `json:"createdAt"`
}

// ListCertificatesResp 是 Certificate 列表接口响应
type ListCertificatesResp struct {
	Certificates []Certificate `json:"certificates"`
}

// GetCertificateResp 是 Certificate 详情接口响应
type GetCertificateResp struct {
	Certificate Certificate `json:"certificate"`
}

// CreateCertificateResp 是创建 Certificate 的响应
type CreateCertificateResp struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
}

// UpdateCertificateResp 是更新 Certificate 的响应
type UpdateCertificateResp struct {
	Success bool `json:"success"`
}

// DeleteCertificateResp 是删除 Certificate 的响应
type DeleteCertificateResp struct {
	Success bool `json:"success"`
}

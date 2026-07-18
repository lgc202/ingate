// Package certificate 实现 Certificate 管理用例
package certificate

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// ListResult 是 Certificate 列表用例结果
type ListResult struct {
	Certificates []resource.Certificate
}

// CertificateResult 是单个 Certificate 用例结果
type CertificateResult struct {
	Certificate *resource.Certificate
}

// CreateParams 是创建 Certificate 用例参数
type CreateParams struct {
	CertificateParams
}

// UpdateParams 是更新 Certificate 用例参数
type UpdateParams struct {
	Version string
	CertificateParams
}

// CertificateParams 是创建和更新 Certificate 共用的配置参数
type CertificateParams struct {
	Name           string
	Description    string
	CertificatePEM string
	PrivateKeyPEM  string
}

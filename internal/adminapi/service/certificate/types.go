// Package certificate 实现 Certificate 管理用例
package certificate

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

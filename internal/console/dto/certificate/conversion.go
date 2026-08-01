package certificate

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// Spec 将已校验的创建请求转换为声明式 CertificateSpec
func (r CreateCertificateReq) Spec() resource.CertificateSpec {
	return r.CertificateConfig.spec()
}

// Spec 将已校验的更新请求转换为声明式 CertificateSpec
func (r UpdateCertificateReq) Spec() resource.CertificateSpec {
	return r.CertificateConfig.spec()
}

func (c CertificateConfig) spec() resource.CertificateSpec {
	return resource.CertificateSpec{
		DisplayName:    c.Name,
		Description:    c.Description,
		CertificatePEM: c.CertificatePEM,
		PrivateKeyPEM:  c.PrivateKeyPEM,
	}
}

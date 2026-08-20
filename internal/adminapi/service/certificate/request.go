package certificate

import (
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// createSpec 把创建请求转换为包含完整密钥对的 Certificate 配置
func createSpec(request *adminv1.CreateCertificateRequest) (resource.CertificateSpec, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return resource.CertificateSpec{}, adminservice.BadRequest("证书名称不能为空")
	}
	certificatePEM, privateKeyPEM, err := normalizeKeyPair(request.GetCertificatePem(), request.GetPrivateKeyPem())
	if err != nil {
		return resource.CertificateSpec{}, err
	}
	return resource.CertificateSpec{
		DisplayName:    name,
		CertificatePEM: certificatePEM,
		PrivateKeyPEM:  privateKeyPEM,
	}, nil
}

// updateSpec 把更新请求转换为 Certificate 配置，未提供密钥对时由 Biz 保留现有内容
func updateSpec(request *adminv1.UpdateCertificateRequest) (resource.CertificateSpec, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return resource.CertificateSpec{}, adminservice.BadRequest("证书名称不能为空")
	}
	if (request.CertificatePem == nil) != (request.PrivateKeyPem == nil) {
		return resource.CertificateSpec{}, adminservice.BadRequest("替换证书时必须同时提供证书内容和私钥")
	}

	spec := resource.CertificateSpec{DisplayName: name}
	if request.CertificatePem == nil {
		return spec, nil
	}
	certificatePEM, privateKeyPEM, err := normalizeKeyPair(request.GetCertificatePem(), request.GetPrivateKeyPem())
	if err != nil {
		return resource.CertificateSpec{}, err
	}
	spec.CertificatePEM = certificatePEM
	spec.PrivateKeyPEM = privateKeyPEM
	return spec, nil
}

func normalizeKeyPair(certificatePEM, privateKeyPEM string) (string, string, error) {
	certificatePEM = normalizePEM(certificatePEM)
	privateKeyPEM = normalizePEM(privateKeyPEM)
	if certificatePEM == "" || privateKeyPEM == "" {
		return "", "", adminservice.BadRequest("证书内容和私钥不能为空")
	}
	if _, err := certificateutil.ParseKeyPair(certificatePEM, privateKeyPEM); err != nil {
		return "", "", adminservice.BadRequest("证书内容与私钥格式不正确或不匹配")
	}
	return certificatePEM, privateKeyPEM, nil
}

func normalizePEM(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return value + "\n"
}

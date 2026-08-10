package certificate

import (
	"strings"

	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// buildCertificateSpec 校验名称和可选的完整证书替换内容
func buildCertificateSpec(name string, certificatePEM, privateKeyPEM *string) (resource.CertificateSpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.CertificateSpec{}, adminservice.BadRequest("证书名称不能为空")
	}
	if (certificatePEM == nil) != (privateKeyPEM == nil) {
		return resource.CertificateSpec{}, adminservice.BadRequest("替换证书时必须同时提供证书内容和私钥")
	}

	spec := resource.CertificateSpec{DisplayName: name}
	if certificatePEM == nil {
		return spec, nil
	}
	spec.CertificatePEM = normalizePEM(*certificatePEM)
	spec.PrivateKeyPEM = normalizePEM(*privateKeyPEM)
	if spec.CertificatePEM == "" || spec.PrivateKeyPEM == "" {
		return resource.CertificateSpec{}, adminservice.BadRequest("证书内容和私钥不能为空")
	}
	if _, err := certificateutil.ParseKeyPair(spec.CertificatePEM, spec.PrivateKeyPEM); err != nil {
		return resource.CertificateSpec{}, adminservice.BadRequest("证书内容与私钥格式不正确或不匹配")
	}
	return spec, nil
}

func normalizePEM(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return value + "\n"
}

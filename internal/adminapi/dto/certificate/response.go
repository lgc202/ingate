package certificate

import (
	"time"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	certificateservice "github.com/lgc202/ingate/internal/adminapi/service/certificate"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// NewListCertificatesResp 转换 Certificate 列表用例结果为 HTTP 响应
func NewListCertificatesResp(result *certificateservice.ListResult) ListCertificatesResp {
	certificates := make([]Certificate, 0, len(result.Certificates))
	for i := range result.Certificates {
		certificates = append(certificates, certificateFromResource(&result.Certificates[i], false))
	}
	return ListCertificatesResp{Certificates: certificates}
}

// NewGetCertificateResp 转换单个 Certificate 用例结果为 HTTP 响应
func NewGetCertificateResp(result *certificateservice.CertificateResult) GetCertificateResp {
	return GetCertificateResp{Certificate: certificateFromResource(result.Certificate, true)}
}

func certificateFromResource(certificate *resource.Certificate, includePEM bool) Certificate {
	result := Certificate{
		ID:          certificate.Name,
		Version:     certificate.ResourceVersion,
		Status:      admindto.NewResourceStatus(certificate.Generation, certificate.Status.Conditions),
		Name:        certificate.Spec.DisplayName,
		Description: certificate.Spec.Description,
		DNSNames:    []string{},
		CreatedAt:   formatTime(certificate.CreationTimestamp.Time),
	}
	if includePEM {
		result.CertificatePEM = certificate.Spec.CertificatePEM
	}
	leaf, err := certificateutil.ParseKeyPair(certificate.Spec.CertificatePEM, certificate.Spec.PrivateKeyPEM)
	if err != nil {
		return result
	}
	result.DNSNames = append([]string(nil), leaf.DNSNames...)
	result.NotBefore = formatTime(leaf.NotBefore)
	result.NotAfter = formatTime(leaf.NotAfter)
	return result
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

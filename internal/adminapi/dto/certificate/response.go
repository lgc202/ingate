package certificate

import (
	"strconv"
	"time"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	"github.com/lgc202/ingate/internal/adminapi/service/resourcestatus"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// NewListCertificatesResp 转换 Certificate 资源列表为 HTTP 响应
func NewListCertificatesResp(resources []resource.Certificate) ListCertificatesResp {
	certificates := make([]Certificate, 0, len(resources))
	for i := range resources {
		certificates = append(certificates, certificateFromResource(&resources[i], false))
	}
	return ListCertificatesResp{Certificates: certificates}
}

// NewGetCertificateResp 转换 Certificate 资源为 HTTP 响应
func NewGetCertificateResp(certificate *resource.Certificate) GetCertificateResp {
	return GetCertificateResp{Certificate: certificateFromResource(certificate, true)}
}

func certificateFromResource(certificate *resource.Certificate, includePEM bool) Certificate {
	result := Certificate{
		ID:          certificate.Name,
		Version:     strconv.FormatInt(certificate.Generation, 10),
		Status:      admindto.NewResourceStatus(resourcestatus.FromConditions(certificate.Generation, certificate.Status.Conditions)),
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

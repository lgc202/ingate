package certificate

import (
	"time"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func certificateFromResource(certificate *resource.Certificate) *adminv1.Certificate {
	status := biz.ResourceStatusFromConditions(certificate.Generation, certificate.Status.Conditions)
	response := &adminv1.Certificate{
		Id:        certificate.Name,
		Name:      certificate.Spec.DisplayName,
		State:     adminservice.NewResourceState(status.State),
		Message:   adminservice.ResourceMessage(status.Reason),
		Version:   certificate.Generation,
		CreatedAt: adminservice.NewTimestamp(certificate.CreationTimestamp.Time),
		UpdatedAt: adminservice.NewTimestamp(certificateUpdatedAt(certificate)),
	}
	leaf, err := certificateutil.ParseKeyPair(certificate.Spec.CertificatePEM, certificate.Spec.PrivateKeyPEM)
	if err == nil {
		response.DnsNames = append([]string(nil), leaf.DNSNames...)
		response.NotBefore = adminservice.NewTimestamp(leaf.NotBefore)
		response.NotAfter = adminservice.NewTimestamp(leaf.NotAfter)
	}
	return response
}

func certificateWithPEMFromResource(certificate *resource.Certificate) *adminv1.Certificate {
	response := certificateFromResource(certificate)
	response.CertificatePem = certificate.Spec.CertificatePEM
	return response
}

func certificateUpdatedAt(certificate *resource.Certificate) time.Time {
	value := certificate.Annotations[resource.AnnotationUpdatedAt]
	if value == "" {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}

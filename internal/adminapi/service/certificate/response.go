package certificate

import (
	"slices"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/resourceview"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service/protocol"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
)

func certificateSummaryResponse(certificate *resource.Certificate) *adminv1.Certificate {
	status := resourceview.StatusFromConditions(
		certificate.Generation,
		certificate.Status.Conditions,
	)
	response := &adminv1.Certificate{
		Id:        certificate.Name,
		Name:      certificate.Spec.DisplayName,
		State:     adminservice.ResourceState(status.State),
		Message:   adminservice.ResourceMessage(status.Reason),
		Version:   certificate.Generation,
		CreatedAt: adminservice.Timestamp(certificate.CreationTimestamp.Time),
		UpdatedAt: adminservice.Timestamp(
			adminservice.ResourceUpdatedAt(certificate.Annotations),
		),
	}
	leaf, err := certificateutil.ParseLeafCertificate(certificate.Spec.CertificatePEM)
	if err == nil {
		response.DnsNames = slices.Clone(leaf.DNSNames)
		response.NotBefore = adminservice.Timestamp(leaf.NotBefore)
		response.NotAfter = adminservice.Timestamp(leaf.NotAfter)
	}
	return response
}

func certificateDetailResponse(certificate *resource.Certificate) *adminv1.Certificate {
	response := certificateSummaryResponse(certificate)
	response.CertificatePem = certificate.Spec.CertificatePEM
	return response
}

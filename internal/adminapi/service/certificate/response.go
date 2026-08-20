package certificate

import (
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func certificateResponse(certificate *resource.Certificate) *adminv1.Certificate {
	status := biz.ResourceStatusFromConditions(certificate.Generation, certificate.Status.Conditions)
	response := &adminv1.Certificate{
		Id:        certificate.Name,
		Name:      certificate.Spec.DisplayName,
		State:     adminservice.ResourceState(status.State),
		Message:   adminservice.ResourceMessage(status.Reason),
		Version:   certificate.Generation,
		CreatedAt: adminservice.Timestamp(certificate.CreationTimestamp.Time),
		UpdatedAt: adminservice.Timestamp(adminservice.ResourceUpdatedAt(certificate.Annotations)),
	}
	leaf, err := certificateutil.ParseKeyPair(certificate.Spec.CertificatePEM, certificate.Spec.PrivateKeyPEM)
	if err == nil {
		response.DnsNames = append([]string(nil), leaf.DNSNames...)
		response.NotBefore = adminservice.Timestamp(leaf.NotBefore)
		response.NotAfter = adminservice.Timestamp(leaf.NotAfter)
	}
	return response
}

func certificateWithPEMResponse(certificate *resource.Certificate) *adminv1.Certificate {
	response := certificateResponse(certificate)
	response.CertificatePem = certificate.Spec.CertificatePEM
	return response
}

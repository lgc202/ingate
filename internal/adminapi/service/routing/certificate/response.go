package certificate

import (
	"slices"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/resource/status"
	"github.com/lgc202/ingate/internal/adminapi/service/conversion"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
)

func certificateSummaryResponse(certificate *resource.Certificate) *adminv1.Certificate {
	resourceStatus := status.FromConditions(
		certificate.Generation,
		certificate.Status.Conditions,
	)
	response := &adminv1.Certificate{
		Id:        certificate.Name,
		Name:      certificate.Spec.DisplayName,
		State:     conversion.ResourceState(resourceStatus.State),
		Message:   conversion.ResourceMessage(resourceStatus.Reason),
		Version:   certificate.Generation,
		CreatedAt: conversion.Timestamp(certificate.CreationTimestamp.Time),
		UpdatedAt: conversion.Timestamp(
			conversion.ResourceUpdatedAt(certificate.Annotations),
		),
	}
	leaf, err := certificateutil.ParseLeafCertificate(certificate.Spec.CertificatePEM)
	if err == nil {
		response.DnsNames = slices.Clone(leaf.DNSNames)
		response.NotBefore = conversion.Timestamp(leaf.NotBefore)
		response.NotAfter = conversion.Timestamp(leaf.NotAfter)
	}
	return response
}

func certificateDetailResponse(certificate *resource.Certificate) *adminv1.Certificate {
	response := certificateSummaryResponse(certificate)
	response.CertificatePem = certificate.Spec.CertificatePEM
	return response
}

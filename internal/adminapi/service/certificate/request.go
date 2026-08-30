package certificate

import (
	"strings"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
)

func parseCertificateSpec(
	displayName string,
	certificatePEM string,
	privateKeyPEM string,
) (resource.CertificateSpec, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return resource.CertificateSpec{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"证书名称不能为空",
		)
	}
	certificatePEM, privateKeyPEM, err := parseKeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		return resource.CertificateSpec{}, err
	}
	return resource.CertificateSpec{
		DisplayName:    displayName,
		CertificatePEM: certificatePEM,
		PrivateKeyPEM:  privateKeyPEM,
	}, nil
}

func parseCertificateReplacement(
	request *adminv1.UpdateCertificateRequest,
) (resource.CertificateSpec, bool, error) {
	displayName := strings.TrimSpace(request.GetName())
	if displayName == "" {
		return resource.CertificateSpec{}, false, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"证书名称不能为空",
		)
	}
	if (request.CertificatePem == nil) != (request.PrivateKeyPem == nil) {
		return resource.CertificateSpec{}, false, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"替换证书时必须同时提供证书内容和私钥",
		)
	}
	if request.CertificatePem == nil {
		return resource.CertificateSpec{DisplayName: displayName}, true, nil
	}

	certificatePEM, privateKeyPEM, err := parseKeyPair(
		request.GetCertificatePem(),
		request.GetPrivateKeyPem(),
	)
	if err != nil {
		return resource.CertificateSpec{}, false, err
	}
	return resource.CertificateSpec{
		DisplayName:    displayName,
		CertificatePEM: certificatePEM,
		PrivateKeyPEM:  privateKeyPEM,
	}, false, nil
}

func parseKeyPair(certificatePEM, privateKeyPEM string) (string, string, error) {
	certificatePEM = certificateutil.NormalizePEM(certificatePEM)
	privateKeyPEM = certificateutil.NormalizePEM(privateKeyPEM)
	if certificatePEM == "" || privateKeyPEM == "" {
		return "", "", errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"证书内容和私钥不能为空",
		)
	}
	if len(certificatePEM) > certificateutil.MaxCertificatePEMBytes {
		return "", "", errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"证书链大小不能超过 256 KiB",
		)
	}
	if len(privateKeyPEM) > certificateutil.MaxPrivateKeyPEMBytes {
		return "", "", errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"私钥大小不能超过 64 KiB",
		)
	}
	if _, err := certificateutil.ParseKeyPair(certificatePEM, privateKeyPEM); err != nil {
		return "", "", errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"证书内容与私钥格式不正确或不匹配",
		).WithCause(err)
	}
	return certificatePEM, privateKeyPEM, nil
}

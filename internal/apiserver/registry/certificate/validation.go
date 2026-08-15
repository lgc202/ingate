package certificate

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	certificateparser "github.com/lgc202/ingate/internal/pkg/certificate"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

func validateCertificate(certificate *resource.Certificate) field.ErrorList {
	specPath := field.NewPath("spec")
	var errs field.ErrorList

	if certificate.Spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	if certificate.Spec.CertificatePEM == "" {
		errs = append(errs, field.Required(specPath.Child("certificatePEM"), "certificatePEM is required"))
	}
	if certificate.Spec.PrivateKeyPEM == "" {
		errs = append(errs, field.Required(specPath.Child("privateKeyPEM"), "privateKeyPEM is required"))
	}
	if certificate.Spec.CertificatePEM == "" || certificate.Spec.PrivateKeyPEM == "" {
		return errs
	}

	if _, err := certificateparser.ParseKeyPair(certificate.Spec.CertificatePEM, certificate.Spec.PrivateKeyPEM); err != nil {
		errs = append(errs, field.Invalid(
			specPath.Child("certificatePEM"),
			"<redacted>",
			"certificate and private key must be valid PEM and match",
		))
	}
	return errs
}

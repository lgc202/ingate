package certificate

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
)

func validateCertificate(certificate *resource.Certificate) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := apiregistry.ValidateResourceID(certificate.Name, field.NewPath("metadata", "name"))

	errs = append(errs, apiregistry.ValidateDisplayName(
		certificate.Spec.DisplayName,
		specPath.Child("displayName"),
	)...)
	if certificate.Spec.CertificatePEM == "" {
		errs = append(errs, field.Required(specPath.Child("certificatePEM"), "certificatePEM is required"))
	}
	if certificate.Spec.PrivateKeyPEM == "" {
		errs = append(errs, field.Required(specPath.Child("privateKeyPEM"), "privateKeyPEM is required"))
	}
	if certificate.Spec.CertificatePEM == "" || certificate.Spec.PrivateKeyPEM == "" {
		return errs
	}
	tooLarge := false
	if len(certificate.Spec.CertificatePEM) > certificateutil.MaxCertificatePEMBytes {
		errs = append(errs, field.Invalid(
			specPath.Child("certificatePEM"),
			"<redacted>",
			fmt.Sprintf("must not exceed %d bytes", certificateutil.MaxCertificatePEMBytes),
		))
		tooLarge = true
	}
	if len(certificate.Spec.PrivateKeyPEM) > certificateutil.MaxPrivateKeyPEMBytes {
		errs = append(errs, field.Invalid(
			specPath.Child("privateKeyPEM"),
			"<redacted>",
			fmt.Sprintf("must not exceed %d bytes", certificateutil.MaxPrivateKeyPEMBytes),
		))
		tooLarge = true
	}
	if tooLarge {
		return errs
	}

	if _, err := certificateutil.ParseKeyPair(
		certificate.Spec.CertificatePEM,
		certificate.Spec.PrivateKeyPEM,
	); err != nil {
		errs = append(errs, field.Invalid(
			specPath.Child("certificatePEM"),
			"<redacted>",
			"certificate and private key must be valid PEM and match",
		))
	}
	return errs
}

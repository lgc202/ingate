package biz

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sort"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/convert"
	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	"github.com/lgc202/ingate/internal/adminapi/store"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type CertificateService struct {
	store store.Store
}

func NewCertificateService(store store.Store) *CertificateService {
	return &CertificateService{store: store}
}

func (s *CertificateService) Create(ctx context.Context, req dto.CreateCertificateRequest) (dto.CertificateResponse, error) {
	source := normalizeCertificateSource(req.Source, req.SecretRef, req.Upload)
	if err := validateCertificateRequest(source, req.SecretRef, req.Upload, false); err != nil {
		return dto.CertificateResponse{}, err
	}

	certificate := convert.CertificateFromCreateRequest(req)
	if source == convert.CertificateSourceUpload {
		managedSecretName := managedSecretNameForCertificate(req.Name)
		secret := buildManagedTLSSecret(req.Name, managedSecretName, req.Upload.CertPEM, req.Upload.KeyPEM)
		if _, err := s.store.CreateSecret(ctx, secret); err != nil {
			return dto.CertificateResponse{}, err
		}
		certificate.Spec.SecretRef = gatewayv1alpha1.LocalObjectReference{Name: managedSecretName}
		convert.SetCertificateSource(certificate, source, managedSecretName)
	} else {
		if _, err := s.store.GetSecret(ctx, req.SecretRef.Name); err != nil {
			return dto.CertificateResponse{}, err
		}
		convert.SetCertificateSource(certificate, source, "")
	}

	created, err := s.store.CreateCertificate(ctx, certificate)
	if err != nil {
		return dto.CertificateResponse{}, err
	}
	return s.toCertificateResponse(ctx, created)
}

func (s *CertificateService) Update(ctx context.Context, name string, req dto.UpdateCertificateRequest) (dto.CertificateResponse, error) {
	current, err := s.store.GetCertificate(ctx, name)
	if err != nil {
		return dto.CertificateResponse{}, err
	}

	source := normalizeCertificateSource(req.Source, req.SecretRef, req.Upload)
	if err := validateCertificateRequest(source, req.SecretRef, req.Upload, convert.CertificateSourceFromObject(current) == convert.CertificateSourceUpload); err != nil {
		return dto.CertificateResponse{}, err
	}

	updated := convert.CertificateFromUpdateRequest(name, req)
	updated.ObjectMeta = current.ObjectMeta
	updated.Status = current.Status
	updated.Annotations = mapsClone(current.Annotations)

	currentSource := convert.CertificateSourceFromObject(current)
	currentManagedSecretName := convert.CertificateManagedSecretName(current)
	if source == convert.CertificateSourceUpload {
		managedSecretName := currentManagedSecretName
		if managedSecretName == "" {
			managedSecretName = managedSecretNameForCertificate(name)
		}
		if req.Upload != nil && (strings.TrimSpace(req.Upload.CertPEM) != "" || strings.TrimSpace(req.Upload.KeyPEM) != "") {
			if err := s.upsertManagedSecret(ctx, name, managedSecretName, req.Upload.CertPEM, req.Upload.KeyPEM); err != nil {
				return dto.CertificateResponse{}, err
			}
		}
		updated.Spec.SecretRef = gatewayv1alpha1.LocalObjectReference{Name: managedSecretName}
		convert.SetCertificateSource(updated, source, managedSecretName)
	} else {
		if _, err := s.store.GetSecret(ctx, req.SecretRef.Name); err != nil {
			return dto.CertificateResponse{}, err
		}
		if currentSource == convert.CertificateSourceUpload && currentManagedSecretName != "" && currentManagedSecretName != req.SecretRef.Name {
			if err := s.store.DeleteSecret(ctx, currentManagedSecretName); err != nil && !apierrors.IsNotFound(err) {
				return dto.CertificateResponse{}, err
			}
		}
		convert.SetCertificateSource(updated, source, "")
	}

	result, err := s.store.UpdateCertificate(ctx, updated)
	if err != nil {
		return dto.CertificateResponse{}, err
	}
	return s.toCertificateResponse(ctx, result)
}

func (s *CertificateService) Delete(ctx context.Context, name string) error {
	current, err := s.store.GetCertificate(ctx, name)
	if err != nil {
		return err
	}
	if err := s.store.DeleteCertificate(ctx, name); err != nil {
		return err
	}
	if convert.CertificateSourceFromObject(current) != convert.CertificateSourceUpload {
		return nil
	}
	if secretName := convert.CertificateManagedSecretName(current); secretName != "" {
		if err := s.store.DeleteSecret(ctx, secretName); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (s *CertificateService) Get(ctx context.Context, name string) (dto.CertificateResponse, error) {
	certificate, err := s.store.GetCertificate(ctx, name)
	if err != nil {
		return dto.CertificateResponse{}, err
	}
	return s.toCertificateResponse(ctx, certificate)
}

func (s *CertificateService) List(ctx context.Context) (dto.CertificateListResponse, error) {
	list, err := s.store.ListCertificates(ctx)
	if err != nil {
		return dto.CertificateListResponse{}, err
	}
	items := make([]dto.CertificateResponse, 0, len(list.Items))
	for i := range list.Items {
		response, responseErr := s.toCertificateResponse(ctx, &list.Items[i])
		if responseErr != nil {
			return dto.CertificateListResponse{}, responseErr
		}
		items = append(items, response)
	}
	return dto.CertificateListResponse{Items: items}, nil
}

func (s *CertificateService) ListSecretOptions(ctx context.Context) (dto.SecretOptionListResponse, error) {
	list, err := s.store.ListSecrets(ctx)
	if err != nil {
		return dto.SecretOptionListResponse{}, err
	}

	items := make([]dto.SecretOption, 0, len(list.Items))
	for _, secret := range list.Items {
		if secret.Spec.Type != "kubernetes.io/tls" {
			continue
		}
		items = append(items, dto.SecretOption{
			Name:            secret.Name,
			Managed:         secret.Labels["app.kubernetes.io/managed-by"] == "ingate-admin-api",
			CertificateName: secret.Labels["gateway.ingate.io/certificate"],
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Managed != items[j].Managed {
			return !items[i].Managed && items[j].Managed
		}
		return items[i].Name < items[j].Name
	})
	return dto.SecretOptionListResponse{Items: items}, nil
}

func normalizeCertificateSource(source string, secretRef *dto.LocalObjectReference, upload *dto.UploadedCertificateInput) string {
	switch source {
	case convert.CertificateSourceUpload, convert.CertificateSourceExistingSecret:
		return source
	}
	if upload != nil {
		return convert.CertificateSourceUpload
	}
	if secretRef != nil && strings.TrimSpace(secretRef.Name) != "" {
		return convert.CertificateSourceExistingSecret
	}
	return convert.CertificateSourceUpload
}

func validateCertificateRequest(source string, secretRef *dto.LocalObjectReference, upload *dto.UploadedCertificateInput, allowEmptyUpload bool) error {
	switch source {
	case convert.CertificateSourceExistingSecret:
		if secretRef == nil || strings.TrimSpace(secretRef.Name) == "" {
			return apierrors.NewBadRequest("existing secret mode requires secretRef.name")
		}
		return nil
	case convert.CertificateSourceUpload:
		if upload == nil {
			if allowEmptyUpload {
				return nil
			}
			return apierrors.NewBadRequest("upload mode requires upload payload")
		}
		certPEM := strings.TrimSpace(upload.CertPEM)
		keyPEM := strings.TrimSpace(upload.KeyPEM)
		if certPEM == "" && keyPEM == "" && allowEmptyUpload {
			return nil
		}
		if certPEM == "" || keyPEM == "" {
			return apierrors.NewBadRequest("upload mode requires both certPEM and keyPEM")
		}
		if _, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM)); err != nil {
			return apierrors.NewBadRequest(fmt.Sprintf("invalid TLS certificate or private key: %v", err))
		}
		return nil
	default:
		return apierrors.NewBadRequest("unsupported certificate source")
	}
}

func (s *CertificateService) upsertManagedSecret(ctx context.Context, certificateName, secretName, certPEM, keyPEM string) error {
	desired := buildManagedTLSSecret(certificateName, secretName, certPEM, keyPEM)
	current, err := s.store.GetSecret(ctx, secretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, createErr := s.store.CreateSecret(ctx, desired)
			return createErr
		}
		return err
	}

	desired.ObjectMeta = current.ObjectMeta
	desired.ResourceVersion = current.ResourceVersion
	_, err = s.store.UpdateSecret(ctx, desired)
	return err
}

func buildManagedTLSSecret(certificateName, secretName, certPEM, keyPEM string) *gatewayv1alpha1.Secret {
	return &gatewayv1alpha1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gatewayv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by":  "ingate-admin-api",
				"gateway.ingate.io/certificate": certificateName,
			},
		},
		Spec: gatewayv1alpha1.SecretSpec{
			Type: "kubernetes.io/tls",
			StringData: map[string]string{
				"tls.crt": strings.TrimSpace(certPEM),
				"tls.key": strings.TrimSpace(keyPEM),
			},
		},
	}
}

func managedSecretNameForCertificate(name string) string {
	const suffix = "-tls"
	const maxLength = 63
	if len(name)+len(suffix) <= maxLength {
		return name + suffix
	}
	trimmed := name[:maxLength-len(suffix)]
	trimmed = strings.TrimRight(trimmed, "-")
	if trimmed == "" {
		return "certificate" + suffix
	}
	return trimmed + suffix
}

func mapsClone(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func (s *CertificateService) toCertificateResponse(ctx context.Context, certificate *gatewayv1alpha1.Certificate) (dto.CertificateResponse, error) {
	response := convert.CertificateToResponse(certificate)
	summary, err := s.buildCertificateSummary(ctx, certificate)
	if err != nil {
		return dto.CertificateResponse{}, err
	}
	response.Spec.Summary = summary
	return response, nil
}

func (s *CertificateService) buildCertificateSummary(ctx context.Context, certificate *gatewayv1alpha1.Certificate) (*dto.CertificateSummary, error) {
	if certificate == nil || certificate.Spec.SecretRef.Name == "" {
		return &dto.CertificateSummary{Status: "MissingSecret"}, nil
	}

	secret, err := s.store.GetSecret(ctx, certificate.Spec.SecretRef.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &dto.CertificateSummary{Status: "MissingSecret"}, nil
		}
		return nil, err
	}
	if secret.Spec.Type != "kubernetes.io/tls" {
		return &dto.CertificateSummary{Status: "Invalid"}, nil
	}
	certPEM := strings.TrimSpace(secret.Spec.StringData["tls.crt"])
	if certPEM == "" {
		return &dto.CertificateSummary{Status: "Invalid"}, nil
	}

	certificatePEMBlock, _ := pem.Decode([]byte(certPEM))
	if certificatePEMBlock == nil || certificatePEMBlock.Type != "CERTIFICATE" {
		return &dto.CertificateSummary{Status: "Invalid"}, nil
	}
	parsed, err := x509.ParseCertificate(certificatePEMBlock.Bytes)
	if err != nil {
		return &dto.CertificateSummary{Status: "Invalid"}, nil
	}

	now := time.Now()
	daysRemaining := int(parsed.NotAfter.Sub(now).Hours() / 24)
	status := "Healthy"
	switch {
	case parsed.NotAfter.Before(now):
		status = "Expired"
	case parsed.NotAfter.Before(now.Add(30 * 24 * time.Hour)):
		status = "ExpiringSoon"
	}

	return &dto.CertificateSummary{
		CommonName:    parsed.Subject.CommonName,
		DNSNames:      append([]string(nil), parsed.DNSNames...),
		ExpiresAt:     parsed.NotAfter.Format(time.RFC3339),
		DaysRemaining: daysRemaining,
		Status:        status,
	}, nil
}

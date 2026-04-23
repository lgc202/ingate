package convert

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

const (
	CertificateSourceUpload         = "Upload"
	CertificateSourceExistingSecret = "ExistingSecret"

	CertificateAnnotationSource            = "adminapi.ingate.io/certificate-source"
	CertificateAnnotationManagedSecretName = "adminapi.ingate.io/managed-secret-name"
)

func CertificateFromCreateRequest(req dto.CreateCertificateRequest) *gatewayv1alpha1.Certificate {
	return &gatewayv1alpha1.Certificate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gatewayv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Certificate",
		},
		ObjectMeta: metav1.ObjectMeta{Name: req.Name},
		Spec: gatewayv1alpha1.CertificateSpec{
			SecretRef: localObjectReferenceValueFromDTOPtr(req.SecretRef),
			Domains:   append([]string(nil), req.Domains...),
		},
	}
}

func CertificateFromUpdateRequest(name string, req dto.UpdateCertificateRequest) *gatewayv1alpha1.Certificate {
	return &gatewayv1alpha1.Certificate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gatewayv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Certificate",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: gatewayv1alpha1.CertificateSpec{
			SecretRef: localObjectReferenceValueFromDTOPtr(req.SecretRef),
			Domains:   append([]string(nil), req.Domains...),
		},
	}
}

func CertificateToResponse(certificate *gatewayv1alpha1.Certificate) dto.CertificateResponse {
	if certificate == nil {
		return dto.CertificateResponse{}
	}
	return dto.CertificateResponse{
		Metadata: dto.NewObjectMeta(certificate.ObjectMeta),
		Spec: dto.CertificateSpec{
			Source:    CertificateSourceFromObject(certificate),
			SecretRef: localObjectReferenceValueToDTO(certificate.Spec.SecretRef),
			Domains:   append([]string(nil), certificate.Spec.Domains...),
		},
		Status: dto.CertificateStatusView{
			ObservedGeneration: certificate.Status.ObservedGeneration,
			Conditions:         dto.NewConditions(certificate.Status.Conditions),
		},
	}
}

func CertificateListToResponse(list *gatewayv1alpha1.CertificateList) dto.CertificateListResponse {
	if list == nil {
		return dto.CertificateListResponse{}
	}
	items := make([]dto.CertificateResponse, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, CertificateToResponse(&list.Items[i]))
	}
	return dto.CertificateListResponse{Items: items}
}

func localObjectReferenceValueFromDTO(ref dto.LocalObjectReference) gatewayv1alpha1.LocalObjectReference {
	return gatewayv1alpha1.LocalObjectReference{Name: ref.Name}
}

func localObjectReferenceValueFromDTOPtr(ref *dto.LocalObjectReference) gatewayv1alpha1.LocalObjectReference {
	if ref == nil {
		return gatewayv1alpha1.LocalObjectReference{}
	}
	return localObjectReferenceValueFromDTO(*ref)
}

func localObjectReferenceValueToDTO(ref gatewayv1alpha1.LocalObjectReference) dto.LocalObjectReference {
	return dto.LocalObjectReference{Name: ref.Name}
}

func CertificateSourceFromObject(certificate *gatewayv1alpha1.Certificate) string {
	if certificate == nil || certificate.Annotations == nil {
		return CertificateSourceExistingSecret
	}
	if source := certificate.Annotations[CertificateAnnotationSource]; source != "" {
		return source
	}
	return CertificateSourceExistingSecret
}

func CertificateManagedSecretName(certificate *gatewayv1alpha1.Certificate) string {
	if certificate == nil || certificate.Annotations == nil {
		return ""
	}
	return certificate.Annotations[CertificateAnnotationManagedSecretName]
}

func SetCertificateSource(certificate *gatewayv1alpha1.Certificate, source, managedSecretName string) {
	if certificate.Annotations == nil {
		certificate.Annotations = map[string]string{}
	}
	certificate.Annotations[CertificateAnnotationSource] = source
	if managedSecretName == "" {
		delete(certificate.Annotations, CertificateAnnotationManagedSecretName)
		return
	}
	certificate.Annotations[CertificateAnnotationManagedSecretName] = managedSecretName
}

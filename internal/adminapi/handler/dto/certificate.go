package dto

type CreateCertificateRequest struct {
	Name      string                    `json:"name" binding:"required"`
	Source    string                    `json:"source,omitempty"`
	SecretRef *LocalObjectReference     `json:"secretRef,omitempty"`
	Upload    *UploadedCertificateInput `json:"upload,omitempty"`
	Domains   []string                  `json:"domains,omitempty"`
}

type UpdateCertificateRequest struct {
	Source    string                    `json:"source,omitempty"`
	SecretRef *LocalObjectReference     `json:"secretRef,omitempty"`
	Upload    *UploadedCertificateInput `json:"upload,omitempty"`
	Domains   []string                  `json:"domains,omitempty"`
}

type CertificateResponse struct {
	Metadata ObjectMeta            `json:"metadata"`
	Spec     CertificateSpec       `json:"spec"`
	Status   CertificateStatusView `json:"status,omitempty"`
}

type CertificateSpec struct {
	Source    string               `json:"source,omitempty"`
	SecretRef LocalObjectReference `json:"secretRef,omitempty"`
	Domains   []string             `json:"domains,omitempty"`
	Summary   *CertificateSummary  `json:"summary,omitempty"`
}

type CertificateStatusView struct {
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

type CertificateListResponse struct {
	Items []CertificateResponse `json:"items"`
}

type UploadedCertificateInput struct {
	CertPEM string `json:"certPEM,omitempty"`
	KeyPEM  string `json:"keyPEM,omitempty"`
}

type CertificateSummary struct {
	CommonName    string   `json:"commonName,omitempty"`
	DNSNames      []string `json:"dnsNames,omitempty"`
	ExpiresAt     string   `json:"expiresAt,omitempty"`
	DaysRemaining int      `json:"daysRemaining,omitempty"`
	Status        string   `json:"status,omitempty"`
}

type SecretOption struct {
	Name            string `json:"name"`
	Managed         bool   `json:"managed"`
	CertificateName string `json:"certificateName,omitempty"`
}

type SecretOptionListResponse struct {
	Items []SecretOption `json:"items"`
}

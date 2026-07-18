package upstreamcredential

import (
	"time"

	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	credentialservice "github.com/lgc202/ingate/internal/adminapi/service/upstreamcredential"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// NewListUpstreamCredentialsResp 转换 UpstreamCredential 列表用例结果为 HTTP 响应
func NewListUpstreamCredentialsResp(result *credentialservice.ListResult) ListUpstreamCredentialsResp {
	credentials := make([]UpstreamCredential, 0, len(result.Credentials))
	for i := range result.Credentials {
		credentials = append(credentials, credentialFromResource(&result.Credentials[i]))
	}
	return ListUpstreamCredentialsResp{Credentials: credentials}
}

// NewGetUpstreamCredentialResp 转换单个 UpstreamCredential 用例结果为 HTTP 响应
func NewGetUpstreamCredentialResp(result *credentialservice.CredentialResult) GetUpstreamCredentialResp {
	return GetUpstreamCredentialResp{Credential: credentialFromResource(result.Credential)}
}

func credentialFromResource(credential *resource.UpstreamCredential) UpstreamCredential {
	return UpstreamCredential{
		ID:         credential.Name,
		Version:    credential.ResourceVersion,
		Status:     admindto.NewResourceStatus(credential.Generation, credential.Status.Conditions),
		Name:       credential.Spec.DisplayName,
		Type:       credential.Spec.Type,
		Configured: credential.Spec.APIKey != nil && credential.Spec.APIKey.Value != "",
		CreatedAt:  formatTime(credential.CreationTimestamp.Time),
	}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

// Package upstreamcredential 实现上游访问凭据管理用例
package upstreamcredential

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// ListResult 是 UpstreamCredential 列表用例结果
type ListResult struct {
	Credentials []resource.UpstreamCredential
}

// CredentialResult 是单个 UpstreamCredential 用例结果
type CredentialResult struct {
	Credential *resource.UpstreamCredential
}

// CreateParams 是创建 UpstreamCredential 用例参数
type CreateParams struct {
	CredentialParams
}

// UpdateParams 是更新 UpstreamCredential 用例参数
type UpdateParams struct {
	Version string
	CredentialParams
}

// CredentialParams 是创建和更新 UpstreamCredential 共用的配置参数
type CredentialParams struct {
	Name        string
	Type        resource.UpstreamCredentialType
	APIKeyValue string
}

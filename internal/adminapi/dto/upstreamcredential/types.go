// Package upstreamcredential 定义上游访问凭据管理接口的请求和响应模型
package upstreamcredential

import (
	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// APIKeyConfig 是控制台写入的 API Key 配置
type APIKeyConfig struct {
	Value string `json:"value"`
}

// UpstreamCredentialConfig 是控制台创建和更新上游访问凭据的产品配置
type UpstreamCredentialConfig struct {
	Name   string                          `json:"name"`
	Type   resource.UpstreamCredentialType `json:"type"`
	APIKey *APIKeyConfig                   `json:"apiKey,omitempty"`
}

// CreateUpstreamCredentialReq 是创建 UpstreamCredential 的请求体
type CreateUpstreamCredentialReq struct {
	UpstreamCredentialConfig
}

// UpdateUpstreamCredentialReq 是更新 UpstreamCredential 的请求体
type UpdateUpstreamCredentialReq struct {
	Version string `json:"version"`
	UpstreamCredentialConfig
}

// IDReq 是访问单个 UpstreamCredential 时的路径参数
type IDReq struct {
	ID string `uri:"id"`
}

// UpstreamCredential 是 admin-api 面向控制台返回的上游访问凭据
type UpstreamCredential struct {
	ID         string                          `json:"id"`
	Version    string                          `json:"version,omitempty"`
	Status     admindto.ResourceStatus         `json:"status"`
	Name       string                          `json:"name"`
	Type       resource.UpstreamCredentialType `json:"type"`
	Configured bool                            `json:"configured"`
	CreatedAt  string                          `json:"createdAt"`
}

// ListUpstreamCredentialsResp 是 UpstreamCredential 列表接口响应
type ListUpstreamCredentialsResp struct {
	Credentials []UpstreamCredential `json:"credentials"`
}

// GetUpstreamCredentialResp 是 UpstreamCredential 详情接口响应
type GetUpstreamCredentialResp struct {
	Credential UpstreamCredential `json:"credential"`
}

// CreateUpstreamCredentialResp 是创建 UpstreamCredential 的响应
type CreateUpstreamCredentialResp struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
}

// UpdateUpstreamCredentialResp 是更新 UpstreamCredential 的响应
type UpdateUpstreamCredentialResp struct {
	Success bool `json:"success"`
}

// DeleteUpstreamCredentialResp 是删除 UpstreamCredential 的响应
type DeleteUpstreamCredentialResp struct {
	Success bool `json:"success"`
}

// Package upstream 定义 Upstream 管理接口的请求和响应模型
package upstream

import (
	admindto "github.com/lgc202/ingate/internal/admin/dto"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// CreateUpstreamReq 是创建 Upstream 的请求体
type CreateUpstreamReq struct {
	UpstreamConfig
	APIKey *APIKeyConfig `json:"apiKey,omitempty"`
}

// UpdateUpstreamReq 是更新 Upstream 的请求体
type UpdateUpstreamReq struct {
	Version      string        `json:"version"`
	APIKey       *APIKeyConfig `json:"apiKey,omitempty"`
	RemoveAPIKey bool          `json:"removeAPIKey,omitempty"`
	UpstreamConfig
}

// UpstreamConfig 是控制台读写 Upstream 时复用的核心配置
type UpstreamConfig struct {
	Name              string                             `json:"name"`
	Type              resource.UpstreamType              `json:"type"`
	Protocol          resource.UpstreamProtocol          `json:"protocol"`
	TLS               *UpstreamTLS                       `json:"tls,omitempty"`
	Model             *ModelConfig                       `json:"model,omitempty"`
	Endpoints         []UpstreamEndpoint                 `json:"endpoints"`
	LoadBalancePolicy resource.UpstreamLoadBalancePolicy `json:"loadBalancePolicy"`
	HealthCheck       *resource.UpstreamHealthCheck      `json:"healthCheck,omitempty"`
}

// APIKeyConfig 是控制台写入的 API Key 配置
type APIKeyConfig struct {
	Value string `json:"value"`
}

// UpstreamTLS 是控制台读写服务 HTTPS 配置的产品模型
type UpstreamTLS struct {
	ServerName string `json:"serverName"`
}

// ModelConfig 是控制台读写的模型厂商和模型目录配置
type ModelConfig struct {
	Provider    resource.ModelProvider `json:"provider"`
	APIBasePath string                 `json:"apiBasePath"`
	Models      []ModelCatalogItem     `json:"models"`
}

// ModelCatalogItem 是控制台维护的一个厂商模型
type ModelCatalogItem struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Enabled     bool   `json:"enabled"`
}

// UpstreamEndpoint 是控制台读写的服务端点配置
type UpstreamEndpoint struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Weight  int    `json:"weight"`
	Enabled bool   `json:"enabled"`
}

// Upstream 是 Admin API 返回的服务对象，不直接暴露 CR 结构
type Upstream struct {
	ID               string                  `json:"id"`
	Version          string                  `json:"version,omitempty"`
	Status           admindto.ResourceStatus `json:"status"`
	APIKeyConfigured bool                    `json:"apiKeyConfigured"`
	UpstreamConfig
	CreatedAt string `json:"createdAt"`
}

// ListUpstreamsResp 是服务列表接口响应
type ListUpstreamsResp struct {
	Upstreams []Upstream `json:"upstreams"`
}

// CreateUpstreamResp 是创建 Upstream 的响应
type CreateUpstreamResp struct {
	Success bool   `json:"success"`
	ID      string `json:"id,omitempty"`
}

// UpdateUpstreamResp 是更新 Upstream 的响应
type UpdateUpstreamResp struct {
	Success bool `json:"success"`
}

// DeleteUpstreamResp 是删除 Upstream 的响应
type DeleteUpstreamResp struct {
	Success bool `json:"success"`
}

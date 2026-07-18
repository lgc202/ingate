// Package gateway 定义 Gateway 管理接口的请求和响应模型
package gateway

import (
	admindto "github.com/lgc202/ingate/internal/adminapi/dto"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// CreateGatewayReq 是创建 Gateway 的请求体
type CreateGatewayReq struct {
	GatewayConfig
}

// UpdateGatewayReq 是更新 Gateway 的请求体
type UpdateGatewayReq struct {
	Version string `json:"version"`
	GatewayConfig
}

// SetGatewayEnabledReq 是启停 Gateway 的请求体
type SetGatewayEnabledReq struct {
	Enabled *bool `json:"enabled"`
}

// GatewayConfig 是控制台读写 Gateway 时复用的核心配置
type GatewayConfig struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Listeners   []GatewayListener `json:"listeners"`
	Hostnames   []string          `json:"hostnames"`
}

// GatewayListener 是控制台可配置的固定数据面入口
type GatewayListener struct {
	Protocol      resource.Protocol `json:"protocol"`
	Port          int               `json:"port"`
	CertificateID string            `json:"certificateID,omitempty"`
}

// Gateway 是 admin-api 面向控制台返回的 Gateway 对象，不直接暴露 CR 结构
type Gateway struct {
	ID      string                  `json:"id"`
	Version string                  `json:"version,omitempty"`
	Status  admindto.ResourceStatus `json:"status"`
	GatewayConfig
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"createdAt"`
}

// ListGatewaysResp 是 Gateway 列表接口响应
type ListGatewaysResp struct {
	Gateways []Gateway `json:"gateways"`
}

// GetGatewayResp 是 Gateway 详情接口响应
type GetGatewayResp struct {
	Gateway Gateway `json:"gateway"`
}

// CreateGatewayResp 是创建 Gateway 的响应
type CreateGatewayResp struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
}

// UpdateGatewayResp 是更新 Gateway 的响应
type UpdateGatewayResp struct {
	Success bool `json:"success"`
}

// SetGatewayEnabledResp 是启停 Gateway 的响应
type SetGatewayEnabledResp struct {
	Success bool `json:"success"`
}

// DeleteGatewayResp 是删除 Gateway 的响应
type DeleteGatewayResp struct {
	Success bool `json:"success"`
}

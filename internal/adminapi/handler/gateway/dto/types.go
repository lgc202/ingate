package dto

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
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	RuntimeGroup string               `json:"runtimeGroup"`
	Listeners    []GatewayListener    `json:"listeners"`
	HostBindings []GatewayHostBinding `json:"hostBindings"`
}

// GatewayListener 是控制台读写的监听器配置
type GatewayListener struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
}

// GatewayHostBinding 是控制台读写的域名绑定配置
type GatewayHostBinding struct {
	Hostname     string      `json:"hostname,omitempty"`
	ListenerRefs []string    `json:"listenerRefs"`
	TLS          *GatewayTLS `json:"tls,omitempty"`
}

// GatewayTLS 是控制台读写的 TLS 配置
type GatewayTLS struct {
	CertificateRef string `json:"certificateRef,omitempty"`
}

// Gateway 是 admin-api 面向控制台返回的 Gateway 对象，不直接暴露 CR 结构
type Gateway struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
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

package dto

// CreateGatewayReq 是创建 Gateway 的请求体
type CreateGatewayReq struct {
	DisplayName  string                  `json:"displayName"`
	Description  string                  `json:"description"`
	RuntimeGroup string                  `json:"runtimeGroup"`
	Listeners    []GatewayListenerReq    `json:"listeners"`
	HostBindings []GatewayHostBindingReq `json:"hostBindings"`
}

// UpdateGatewayReq 是更新 Gateway 的请求体
type UpdateGatewayReq struct {
	Version      string                  `json:"version"`
	DisplayName  string                  `json:"displayName"`
	Description  string                  `json:"description"`
	RuntimeGroup string                  `json:"runtimeGroup"`
	Listeners    []GatewayListenerReq    `json:"listeners"`
	HostBindings []GatewayHostBindingReq `json:"hostBindings"`
}

// SetGatewayEnabledReq 是启停 Gateway 的请求体
type SetGatewayEnabledReq struct {
	Enabled *bool `json:"enabled"`
}

// GatewayListenerReq 是控制台提交的监听器配置
type GatewayListenerReq struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
}

// GatewayHostBindingReq 是控制台提交的域名绑定配置
type GatewayHostBindingReq struct {
	Hostname     string         `json:"hostname,omitempty"`
	ListenerRefs []string       `json:"listenerRefs"`
	TLS          *GatewayTLSReq `json:"tls,omitempty"`
}

// GatewayTLSReq 是控制台提交的 TLS 配置
type GatewayTLSReq struct {
	CertificateRef string `json:"certificateRef,omitempty"`
}

// ListGatewaysResp 是 Gateway 列表接口响应
type ListGatewaysResp struct {
	Gateways []GatewaySummary `json:"gateways"`
}

// GetGatewayResp 是 Gateway 详情接口响应
type GetGatewayResp struct {
	Gateway GatewayDetail `json:"gateway"`
}

// GetGatewayFormOptionsResp 是 Gateway 表单选项接口响应
type GetGatewayFormOptionsResp struct {
	RuntimeGroups []RuntimeGroupOption `json:"runtimeGroups"`
	Certificates  []CertificateOption  `json:"certificates"`
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

// GatewaySummary 是列表页使用的 Gateway 摘要
type GatewaySummary struct {
	ID                 string               `json:"id"`
	Version            string               `json:"version,omitempty"`
	DisplayName        string               `json:"displayName"`
	Description        string               `json:"description"`
	RuntimeGroup       string               `json:"runtimeGroup"`
	RuntimeGroupName   string               `json:"runtimeGroupName"`
	ListenerSummary    string               `json:"listenerSummary"`
	HostBindingSummary string               `json:"hostBindingSummary"`
	Listeners          []GatewayListener    `json:"listeners"`
	HostBindings       []GatewayHostBinding `json:"hostBindings"`
	Enabled            bool                 `json:"enabled"`
	HealthStatus       string               `json:"healthStatus"`
	LastChangedAt      string               `json:"lastChangedAt"`
}

// GatewayDetail 是详情页使用的 Gateway 配置视图
type GatewayDetail struct {
	ID               string               `json:"id"`
	Version          string               `json:"version,omitempty"`
	DisplayName      string               `json:"displayName"`
	Description      string               `json:"description"`
	RuntimeGroup     string               `json:"runtimeGroup"`
	RuntimeGroupName string               `json:"runtimeGroupName"`
	Listeners        []GatewayListener    `json:"listeners"`
	HostBindings     []GatewayHostBinding `json:"hostBindings"`
	Enabled          bool                 `json:"enabled"`
	HealthStatus     string               `json:"healthStatus"`
	LastChangedAt    string               `json:"lastChangedAt"`
}

// GatewayListener 是响应中的监听器配置
type GatewayListener struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
}

// GatewayHostBinding 是响应中的域名绑定配置
type GatewayHostBinding struct {
	Hostname     string      `json:"hostname,omitempty"`
	ListenerRefs []string    `json:"listenerRefs"`
	TLS          *GatewayTLS `json:"tls,omitempty"`
}

// GatewayTLS 是响应中的 TLS 配置
type GatewayTLS struct {
	CertificateRef string `json:"certificateRef,omitempty"`
}

// RuntimeGroupOption 是 Gateway 表单中的运行组选项
type RuntimeGroupOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CertificateOption 是 Gateway 表单中的证书选项
type CertificateOption struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Domains   []string `json:"domains"`
	ExpiresAt string   `json:"expiresAt"`
	Status    string   `json:"status"`
}

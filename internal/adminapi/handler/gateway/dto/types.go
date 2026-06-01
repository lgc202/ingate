package dto

// Gateway 是 admin-api 面向控制台返回的网关对象，不直接暴露 CR 结构。
type Gateway struct {
	ID                    string     `json:"id"`
	Version               string     `json:"version,omitempty"`
	Name                  string     `json:"name"`
	Description           string     `json:"description"`
	RuntimeGroupID        string     `json:"runtimeGroupId"`
	RuntimeGroupName      string     `json:"runtimeGroupName"`
	ListenerSummary       string     `json:"listeners"`
	Listeners             []Listener `json:"listenerItems"`
	HostPolicy            string     `json:"hostPolicy"`
	Hostnames             []string   `json:"hostnames"`
	RouteCount            int        `json:"routeCount"`
	ServiceCount          int        `json:"serviceCount"`
	Enabled               bool       `json:"enabled"`
	RuntimeStatus         string     `json:"runtimeStatus"`
	HealthStatus          string     `json:"healthStatus"`
	LatestSnapshotVersion string     `json:"latestSnapshotVersion,omitempty"`
	LastChangedAt         string     `json:"lastChangedAt"`
}

// Listener 是控制台使用的监听器信息。
type Listener struct {
	ID              string `json:"id"`
	Protocol        string `json:"protocol"`
	Port            string `json:"port"`
	CertificateID   string `json:"certificateId,omitempty"`
	CertificateName string `json:"certificateName,omitempty"`
}

// ListResponse 是 Gateway 列表接口响应。
type ListResponse struct {
	Gateways     []Gateway     `json:"gateways"`
	Certificates []Certificate `json:"certificates"`
}

// DetailResponse 是 Gateway 详情聚合接口响应。
type DetailResponse struct {
	Gateway          Gateway            `json:"gateway"`
	Routes           []RouteReference   `json:"routes"`
	Services         []ServiceReference `json:"services"`
	RuntimeSnapshots []RuntimeStatus    `json:"runtimeSnapshots"`
}

// Certificate 是控制台证书下拉框所需信息。
type Certificate struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Domains   []string `json:"domains"`
	ExpiresAt string   `json:"expiresAt"`
	Status    string   `json:"status"`
}

// RouteReference 是 Gateway 详情页展示的关联路由。
type RouteReference struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Methods     []string `json:"methods"`
	Path        string   `json:"path"`
	Hostnames   []string `json:"hostnames"`
	ServiceName string   `json:"serviceName"`
}

// ServiceReference 是 Gateway 详情页展示的关联服务。
type ServiceReference struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
}

// RuntimeStatus 是 Gateway 详情页展示的运行配置状态。
type RuntimeStatus struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Target    string `json:"target"`
	Version   string `json:"version"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

// GatewayRequest 是控制台创建或编辑 Gateway 的请求体。
type GatewayRequest struct {
	ID               string            `json:"id,omitempty"`
	Version          string            `json:"version,omitempty"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	RuntimeGroupID   string            `json:"runtimeGroupId"`
	RuntimeGroupName string            `json:"runtimeGroupName"`
	Listeners        []ListenerRequest `json:"listeners"`
	Hostnames        []string          `json:"hostnames"`
	Enabled          *bool             `json:"enabled,omitempty"`
}

// ListenerRequest 是控制台提交的监听器配置。
type ListenerRequest struct {
	ID              string `json:"id"`
	Protocol        string `json:"protocol"`
	Port            string `json:"port"`
	CertificateID   string `json:"certificateId,omitempty"`
	CertificateName string `json:"certificateName,omitempty"`
}

// EnabledRequest 是控制台启停 Gateway 的请求体。
type EnabledRequest struct {
	Enabled *bool `json:"enabled"`
}

// MutationResponse 是 Gateway 变更接口响应。
type MutationResponse struct {
	Success bool `json:"success"`
}

package dto

// ServiceType 是服务分类的稳定接口枚举值
type ServiceType string

const (
	// ServiceTypeApplication 表示普通应用服务
	ServiceTypeApplication ServiceType = "application"
	// ServiceTypeModel 表示模型服务
	ServiceTypeModel ServiceType = "model"
	// ServiceTypeAgent 表示 Agent 服务
	ServiceTypeAgent ServiceType = "agent"
	// ServiceTypeMCP 表示 MCP 服务
	ServiceTypeMCP ServiceType = "mcp"
)

// LoadBalancePolicy 是服务负载均衡策略的稳定接口枚举值
type LoadBalancePolicy string

const (
	// LoadBalancePolicyRoundRobin 表示轮询
	LoadBalancePolicyRoundRobin LoadBalancePolicy = "round_robin"
	// LoadBalancePolicyLeastRequest 表示最少请求
	LoadBalancePolicyLeastRequest LoadBalancePolicy = "least_request"
	// LoadBalancePolicyRandom 表示随机
	LoadBalancePolicyRandom LoadBalancePolicy = "random"
)

// ListResponse 是服务列表接口响应
type ListResponse struct {
	Services  []Upstream        `json:"services"`
	Health    []CountSegment    `json:"health"`
	Incidents []ServiceIncident `json:"incidents"`
}

// Upstream 是 admin-api 面向控制台返回的服务对象，不直接暴露 CR 结构
type Upstream struct {
	ID               string            `json:"id"`
	Version          string            `json:"version,omitempty"`
	Name             string            `json:"name"`
	Type             ServiceType       `json:"type"`
	Endpoint         string            `json:"endpoint"`
	Instances        string            `json:"instances"`
	HealthStatus     string            `json:"healthStatus"`
	RuntimeStatus    string            `json:"runtimeStatus"`
	ReferencedRoutes int               `json:"referencedRoutes"`
	Traffic          string            `json:"traffic"`
	SuccessRate      string            `json:"successRate"`
	LastUpdatedAt    string            `json:"lastUpdatedAt"`
	Endpoints        []EndpointRequest `json:"endpoints"`
}

// CountSegment 是控制台统计块
type CountSegment struct {
	Label string `json:"label"`
	Value int    `json:"value"`
	Tone  string `json:"tone"`
}

// ServiceIncident 是服务异常摘要
type ServiceIncident struct {
	ServiceName string `json:"serviceName"`
	Description string `json:"description"`
	Time        string `json:"time"`
	Status      string `json:"status"`
}

// UpstreamRequest 是控制台创建或编辑服务的请求体
type UpstreamRequest struct {
	ID                         string            `json:"id,omitempty"`
	Version                    string            `json:"version,omitempty"`
	Name                       string            `json:"name"`
	Type                       ServiceType       `json:"type"`
	Endpoint                   string            `json:"endpoint"`
	Instances                  string            `json:"instances"`
	Endpoints                  []EndpointRequest `json:"endpoints"`
	LoadBalancePolicy          LoadBalancePolicy `json:"loadBalancePolicy"`
	HealthCheckEnabled         bool              `json:"healthCheckEnabled"`
	HealthCheckPath            string            `json:"healthCheckPath"`
	HealthCheckIntervalSeconds string            `json:"healthCheckIntervalSeconds"`
	HealthCheckTimeoutSeconds  string            `json:"healthCheckTimeoutSeconds"`
}

// EndpointRequest 是控制台提交的服务端点配置
type EndpointRequest struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Port    string `json:"port"`
	Weight  string `json:"weight"`
	Enabled bool   `json:"enabled"`
}

// MutationResponse 是服务变更接口响应
type MutationResponse struct {
	Success bool `json:"success"`
}

type healthCheckAnnotation struct {
	Enabled         bool   `json:"enabled"`
	Path            string `json:"path"`
	IntervalSeconds int    `json:"intervalSeconds"`
	TimeoutSeconds  int    `json:"timeoutSeconds"`
}

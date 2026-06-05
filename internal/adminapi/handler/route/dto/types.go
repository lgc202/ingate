package dto

// HTTPMethod 是控制台支持的 HTTP 方法
type HTTPMethod string

const (
	// HTTPMethodGET 表示 GET 方法
	HTTPMethodGET HTTPMethod = "GET"
	// HTTPMethodPOST 表示 POST 方法
	HTTPMethodPOST HTTPMethod = "POST"
	// HTTPMethodPUT 表示 PUT 方法
	HTTPMethodPUT HTTPMethod = "PUT"
	// HTTPMethodPATCH 表示 PATCH 方法
	HTTPMethodPATCH HTTPMethod = "PATCH"
	// HTTPMethodDELETE 表示 DELETE 方法
	HTTPMethodDELETE HTTPMethod = "DELETE"
)

// ListResponse 是 Route 列表响应
type ListResponse struct {
	Routes []Route `json:"routes"`
}

// Route 是 admin-api 面向控制台返回的路由对象，不直接暴露 CR 结构
type Route struct {
	ID             string                 `json:"id"`
	Version        string                 `json:"version,omitempty"`
	Methods        []HTTPMethod           `json:"methods"`
	Path           string                 `json:"path"`
	GatewayNames   []string               `json:"gatewayNames"`
	Hostnames      []string               `json:"hostnames"`
	ServiceName    string                 `json:"serviceName"`
	Targets        []TargetService        `json:"targets"`
	PolicyCount    int                    `json:"policyCount"`
	PolicyBindings []PolicyBindingRequest `json:"policyBindings,omitempty"`
	Traffic        string                 `json:"traffic"`
	SuccessRate    string                 `json:"successRate"`
	Enabled        bool                   `json:"enabled"`
	RuntimeStatus  string                 `json:"runtimeStatus"`
	LastChangedAt  string                 `json:"lastChangedAt"`
}

// PolicyCapabilitiesResponse 是当前后端支持的路由策略能力响应
type PolicyCapabilitiesResponse struct {
	Policies []PolicyOption `json:"policies"`
}

// PolicyOption 是路由策略候选项
type PolicyOption struct {
	Capability  RoutePolicyCapability `json:"capability"`
	DisplayName string                `json:"displayName"`
	Meta        string                `json:"meta"`
	Enabled     bool                  `json:"enabled"`
	Params      []PolicyParam         `json:"params"`
}

// PolicyParam 是路由策略参数定义
type PolicyParam struct {
	Key          string   `json:"key"`
	Label        string   `json:"label"`
	DefaultValue string   `json:"defaultValue"`
	InputType    string   `json:"inputType,omitempty"`
	Placeholder  string   `json:"placeholder,omitempty"`
	Required     bool     `json:"required,omitempty"`
	Options      []string `json:"options,omitempty"`
	Unit         string   `json:"unit,omitempty"`
	Min          int      `json:"min,omitempty"`
	Max          int      `json:"max,omitempty"`
}

// RouteRequest 是控制台创建或编辑 Route 的请求体
type RouteRequest struct {
	ID             string                 `json:"id,omitempty"`
	Version        string                 `json:"version,omitempty"`
	Methods        []HTTPMethod           `json:"methods"`
	Path           string                 `json:"path"`
	GatewayNames   []string               `json:"gatewayNames"`
	Hostnames      []string               `json:"hostnames"`
	ServiceName    string                 `json:"serviceName"`
	Targets        []TargetService        `json:"targets,omitempty"`
	Enabled        bool                   `json:"enabled"`
	PolicyBindings []PolicyBindingRequest `json:"policyBindings"`
}

// TargetService 是路由转发到的目标服务及其权重
type TargetService struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

// PolicyBindingRequest 是控制台提交的路由级策略绑定
type PolicyBindingRequest struct {
	Capability RoutePolicyCapability `json:"capability"`
	Source     RoutePolicySource     `json:"source"`
	Parameters map[string]any        `json:"parameters"`
}

// EnabledRequest 是控制台启停 Route 的请求体
type EnabledRequest struct {
	Enabled *bool `json:"enabled"`
}

// MutationResponse 是 Route 变更接口响应
type MutationResponse struct {
	Success bool `json:"success"`
}

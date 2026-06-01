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

// WorkspaceResponse 是路由页面工作区响应
type WorkspaceResponse struct {
	Routes         []Route        `json:"routes"`
	Composer       Composer       `json:"composer"`
	PublishPreview PublishPreview `json:"publishPreview"`
	Detail         Detail         `json:"detail"`
}

// Route 是 admin-api 面向控制台返回的路由对象，不直接暴露 CR 结构
type Route struct {
	ID            string       `json:"id"`
	Version       string       `json:"version,omitempty"`
	Methods       []HTTPMethod `json:"methods"`
	Path          string       `json:"path"`
	GatewayNames  []string     `json:"gatewayNames"`
	Hostnames     []string     `json:"hostnames"`
	ServiceName   string       `json:"serviceName"`
	PolicyCount   int          `json:"policyCount"`
	Traffic       string       `json:"traffic"`
	SuccessRate   string       `json:"successRate"`
	Enabled       bool         `json:"enabled"`
	RuntimeStatus string       `json:"runtimeStatus"`
	LastChangedAt string       `json:"lastChangedAt"`
}

// Composer 是创建或编辑路由所需的候选数据
type Composer struct {
	Methods      []HTTPMethod   `json:"methods"`
	Path         string         `json:"path"`
	GatewayNames []string       `json:"gatewayNames"`
	Hostnames    []string       `json:"hostnames"`
	ServiceName  string         `json:"serviceName"`
	PolicyCount  int            `json:"policyCount"`
	RateLimit    string         `json:"rateLimit"`
	Validations  []string       `json:"validations"`
	Targets      []TargetOption `json:"targets"`
	Policies     []PolicyOption `json:"policies"`
}

// TargetOption 是路由目标服务候选项
type TargetOption struct {
	Name             string `json:"name"`
	Type             string `json:"type"`
	Endpoint         string `json:"endpoint,omitempty"`
	Meta             string `json:"meta"`
	HealthStatus     string `json:"healthStatus"`
	ReferencedRoutes int    `json:"referencedRoutes,omitempty"`
}

// PolicyOption 是路由策略候选项
type PolicyOption struct {
	Name    string        `json:"name"`
	Meta    string        `json:"meta"`
	Enabled bool          `json:"enabled"`
	Params  []PolicyParam `json:"params"`
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

// PublishPreview 是兼容前端现有结构的保存预览数据
type PublishPreview struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Diffs    []Diff `json:"diffs"`
}

// Diff 是预览差异项
type Diff struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

// Detail 是路由详情页默认数据
type Detail struct {
	Title string                `json:"title"`
	Tabs  map[string][]KeyValue `json:"tabs"`
}

// KeyValue 是详情页键值数据
type KeyValue struct {
	Label string `json:"label"`
	Value string `json:"value"`
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
	Enabled        bool                   `json:"enabled"`
	PolicyBindings []PolicyBindingRequest `json:"policyBindings"`
}

// PolicyBindingRequest 是控制台提交的路由级策略绑定
type PolicyBindingRequest struct {
	PolicyName string         `json:"policyName"`
	Source     string         `json:"source"`
	Parameters map[string]any `json:"parameters"`
}

// EnabledRequest 是控制台启停 Route 的请求体
type EnabledRequest struct {
	Enabled *bool `json:"enabled"`
}

// MutationResponse 是 Route 变更接口响应
type MutationResponse struct {
	Success bool `json:"success"`
}

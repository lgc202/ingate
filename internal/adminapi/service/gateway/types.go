package gateway

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

const (
	// DefaultRuntimeGroupID 是第一阶段内置的数据面运行组标识
	DefaultRuntimeGroupID   = "default"
	defaultRuntimeGroupName = "默认运行组"
)

// ListResult 是 Gateway 列表用例结果
type ListResult struct {
	Gateways      []resource.Gateway
	RuntimeGroups []RuntimeGroupOption
}

// GatewayResult 是单个 Gateway 用例结果
type GatewayResult struct {
	Gateway       *resource.Gateway
	RuntimeGroups []RuntimeGroupOption
}

// FormOptionsResult 是 Gateway 表单选项用例结果
type FormOptionsResult struct {
	RuntimeGroups []RuntimeGroupOption
	Certificates  []CertificateOption
}

// RuntimeGroupOption 表示可选择的数据面运行组
type RuntimeGroupOption struct {
	ID   string
	Name string
}

// CertificateOption 表示可选择的证书资源
type CertificateOption struct {
	ID        string
	Name      string
	Domains   []string
	ExpiresAt string
	Status    string
}

// CreateGatewayParams 是创建 Gateway 用例参数
type CreateGatewayParams struct {
	DisplayName  string
	Description  string
	RuntimeGroup string
	Listeners    []ListenerParams
	HostBindings []HostBindingParams
}

// UpdateGatewayParams 是更新 Gateway 用例参数
type UpdateGatewayParams struct {
	Version      string
	DisplayName  string
	Description  string
	RuntimeGroup string
	Listeners    []ListenerParams
	HostBindings []HostBindingParams
}

// ListenerParams 是监听器用例参数
type ListenerParams struct {
	Name     string
	Protocol resource.ListenerProtocol
	Port     int
}

// HostBindingParams 是域名绑定用例参数
type HostBindingParams struct {
	Hostname       string
	ListenerRefs   []string
	CertificateRef string
}

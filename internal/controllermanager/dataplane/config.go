// Package dataplane 定义 Controller 交给数据面适配器的已接纳配置。
package dataplane

// Config 是一次完整发布的期望配置；空集合表示移除已发布的资源。
type Config struct {
	Listeners []Listener
	Backends  []Backend
}

// Listener 是共享数据面上独占一个端口的 HTTP 入口。
type Listener struct {
	Port  int32
	Hosts []Host
}

// Host 是一个 Gateway Listener 占用的主机名及其路由。
type Host struct {
	Name     string
	Hostname string

	// Routes 已按创建时间、资源名称和规则列表顺序排列；适配器稳定排序路径优先级。
	Routes []Route
}

// Route 将一个 Host 上的路径匹配关联到后端。
// PathPrefix 的 Path 已去除尾斜杠，根路径除外。
type Route struct {
	Name     string
	PathType PathMatchType
	Path     string
	// BackendName 为空时保持匹配并返回 HTTP 500，不转发到其他规则。
	BackendName string
}

// PathMatchType 是数据面使用的路径匹配方式。
type PathMatchType string

const (
	// PathMatchExact 匹配完整路径。
	PathMatchExact PathMatchType = "Exact"
	// PathMatchPrefix 按路径段匹配前缀。
	PathMatchPrefix PathMatchType = "PathPrefix"
)

// Backend 是一组可供 Route 引用的后端地址。
type Backend struct {
	Name      string
	Endpoints []Endpoint
}

// Endpoint 是一个后端地址及其相对权重。
type Endpoint struct {
	Address string
	Port    int32
	Weight  int32
}

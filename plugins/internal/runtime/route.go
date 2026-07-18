package runtime

// RouteKey 是内置插件匹配 xDS route identity 的通用 key
type RouteKey struct {
	GatewayName string
	RouteName   string
	RuleName    string
	ConfigID    string
}

// RouteIndex 保存插件编译后的 route 执行配置
type RouteIndex[T any] struct {
	routes map[RouteKey]T
}

// NewRouteIndex 基于 route key 构建查找索引
func NewRouteIndex[T any](routes []T, keyFunc func(T) RouteKey) RouteIndex[T] {
	index := RouteIndex[T]{
		routes: make(map[RouteKey]T, len(routes)),
	}
	for _, route := range routes {
		index.routes[keyFunc(route)] = route
	}
	return index
}

// Get 根据 route key 返回编译后的 route 配置
func (i RouteIndex[T]) Get(key RouteKey) (T, bool) {
	route, ok := i.routes[key]
	return route, ok
}

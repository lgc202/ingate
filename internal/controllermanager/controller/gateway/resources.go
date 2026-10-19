package gateway

import (
	"cmp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// resources 保存本轮读取的只读派生结果；不同集合的读取不构成跨资源事务。
type resources struct {
	gateways    gatewayv1alpha1.GatewayList
	httpRoutes  []*httpRoute
	backends    []*networkBackend
	aiBackends  []*modelBackendBinding
	modelRoutes []*modelRoute
	mcpRoutes   []*mcpRoute
}

// resolution 是本轮配置及父引用状态，不修改输入，也不代表数据面已 ACK。
type resolution struct {
	gateways         []*gatewayv1alpha1.Gateway
	backends         []*networkBackend
	bindings         []httpRouteBinding
	gatewayConflicts []gatewayConflict
	parents          []httpRouteParentResolution
}

// readResources 只在所有源和绑定完成初始同步后使用，不回退到原始 API 对象重译。
func (s *compilationSource) readResources() resources {
	result := resources{
		httpRoutes:  s.httpRoutes.projected.List(),
		backends:    s.backends.projected.List(),
		aiBackends:  s.bindings.List(),
		modelRoutes: s.modelRoutes.projected.List(),
		mcpRoutes:   s.mcpRoutes.projected.List(),
	}
	for _, gateway := range s.gateways.projected.List() {
		result.gateways.Items = append(result.gateways.Items, *gateway)
	}

	return result
}

// resolve 按创建时间、Namespace 和名称稳定排列候选；引用失效不移除路径匹配。
func resolve(
	gateways []gatewayv1alpha1.Gateway,
	routes []*httpRoute,
	backends []*networkBackend,
) resolution {
	var result resolution
	result.selectGateways(gateways)
	result.selectHTTPRoutes(routes, backends)

	return result
}

func compareMeta(a, b metav1.ObjectMeta) int {
	if order := a.CreationTimestamp.Compare(b.CreationTimestamp.Time); order != 0 {
		return order
	}

	if order := cmp.Compare(a.Namespace, b.Namespace); order != 0 {
		return order
	}

	return cmp.Compare(a.Name, b.Name)
}

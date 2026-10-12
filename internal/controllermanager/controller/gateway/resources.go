package gateway

import (
	"cmp"
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// resources 保存一次协调读取的输入；多次 List 不构成跨资源事务。
type resources struct {
	gateways   gatewayv1alpha1.GatewayList
	httpRoutes gatewayv1alpha1.HTTPRouteList
	backends   gatewayv1alpha1.BackendList
}

// resolution 是本轮配置及父引用状态，不修改输入，也不代表数据面已 ACK。
type resolution struct {
	gateways         []*gatewayv1alpha1.Gateway
	backends         []*gatewayv1alpha1.Backend
	bindings         []httpRouteBinding
	gatewayConflicts []gatewayConflict
	parents          []httpRouteParentResolution
}

func (r *Reconciler) readResources(ctx context.Context) (resources, error) {
	var result resources
	if err := r.client.List(ctx, &result.gateways); err != nil {
		return resources{}, fmt.Errorf("list Gateways: %w", err)
	}
	if err := r.client.List(ctx, &result.httpRoutes); err != nil {
		return resources{}, fmt.Errorf("list HTTPRoutes: %w", err)
	}
	if err := r.client.List(ctx, &result.backends); err != nil {
		return resources{}, fmt.Errorf("list Backends: %w", err)
	}

	return result, nil
}

// resolve 按创建时间和名称稳定排列候选；引用失效不移除路径匹配。
func resolve(
	gateways []gatewayv1alpha1.Gateway,
	routes []gatewayv1alpha1.HTTPRoute,
	backends []gatewayv1alpha1.Backend,
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

	return cmp.Compare(a.Name, b.Name)
}

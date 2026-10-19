package gateway

import (
	"cmp"
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// resources 保存一次协调读取的输入及 Backend 网络结果；Cache 与集合读取不构成跨资源事务。
type resources struct {
	gateways    gatewayv1alpha1.GatewayList
	httpRoutes  gatewayv1alpha1.HTTPRouteList
	backends    []*networkBackend
	aiBackends  gatewayv1alpha1.AIBackendList
	modelRoutes gatewayv1alpha1.ModelRouteList
	mcpRoutes   gatewayv1alpha1.MCPRouteList
}

// resolution 是本轮配置及父引用状态，不修改输入，也不代表数据面已 ACK。
type resolution struct {
	gateways         []*gatewayv1alpha1.Gateway
	backends         []*networkBackend
	bindings         []httpRouteBinding
	gatewayConflicts []gatewayConflict
	parents          []httpRouteParentResolution
}

// readResources 只读取两路输出共有的输入；Gateway 读取失败时不能发布部分配置。
func (r *Reconciler) readResources(ctx context.Context) (resources, error) {
	var result resources
	if err := r.client.List(ctx, &result.gateways); err != nil {
		return resources{}, fmt.Errorf("list Gateways: %w", err)
	}

	// 从同一个集合同时取得身份和网络结果，不再混读 Cache 的原对象与异步派生结果。
	result.backends = r.backends.projected.List()

	return result, nil
}

// readAIResources 的完整读取失败不向调用方暴露部分 AI 输入。
func (r *Reconciler) readAIResources(ctx context.Context, snapshot resources) (resources, error) {
	if err := r.client.List(ctx, &snapshot.aiBackends); err != nil {
		return resources{}, fmt.Errorf("list AIBackends: %w", err)
	}

	if err := r.client.List(ctx, &snapshot.modelRoutes); err != nil {
		return resources{}, fmt.Errorf("list ModelRoutes: %w", err)
	}

	if err := r.client.List(ctx, &snapshot.mcpRoutes); err != nil {
		return resources{}, fmt.Errorf("list MCPRoutes: %w", err)
	}

	return snapshot, nil
}

// resolve 按创建时间、Namespace 和名称稳定排列候选；引用失效不移除路径匹配。
func resolve(
	gateways []gatewayv1alpha1.Gateway,
	routes []gatewayv1alpha1.HTTPRoute,
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

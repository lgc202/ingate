package gateway

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// 运行投影保留引用、排序、删除、转发和状态协调所需的字段，不能写回 API Server。
// Spec 使用生成的深拷贝，避免裁剪展示字段或后续转换修改 Informer 的只读对象。
func projectGateway(gateway *gatewayv1alpha1.Gateway) *gatewayv1alpha1.Gateway {
	spec := gateway.Spec.DeepCopy()
	spec.DisplayName = ""

	return &gatewayv1alpha1.Gateway{ObjectMeta: projectResourceMeta(gateway), Spec: *spec}
}

func projectHTTPRoute(route *gatewayv1alpha1.HTTPRoute) *gatewayv1alpha1.HTTPRoute {
	spec := route.Spec.DeepCopy()
	spec.DisplayName = ""

	return &gatewayv1alpha1.HTTPRoute{ObjectMeta: projectResourceMeta(route), Spec: *spec}
}

func projectBackend(backend *gatewayv1alpha1.Backend) *gatewayv1alpha1.Backend {
	spec := backend.Spec.DeepCopy()
	spec.DisplayName = ""

	return &gatewayv1alpha1.Backend{ObjectMeta: projectResourceMeta(backend), Spec: *spec}
}

func projectAIBackend(backend *gatewayv1alpha1.AIBackend) *gatewayv1alpha1.AIBackend {
	spec := backend.Spec.DeepCopy()
	spec.DisplayName = ""

	return &gatewayv1alpha1.AIBackend{ObjectMeta: projectResourceMeta(backend), Spec: *spec}
}

func projectModelRoute(route *gatewayv1alpha1.ModelRoute) *gatewayv1alpha1.ModelRoute {
	spec := route.Spec.DeepCopy()
	spec.DisplayName = ""

	return &gatewayv1alpha1.ModelRoute{ObjectMeta: projectResourceMeta(route), Spec: *spec}
}

func projectMCPRoute(route *gatewayv1alpha1.MCPRoute) *gatewayv1alpha1.MCPRoute {
	spec := route.Spec.DeepCopy()
	spec.DisplayName = ""

	return &gatewayv1alpha1.MCPRoute{ObjectMeta: projectResourceMeta(route), Spec: *spec}
}

func projectResourceMeta(resource client.Object) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace:         resource.GetNamespace(),
		Name:              resource.GetName(),
		UID:               resource.GetUID(),
		CreationTimestamp: resource.GetCreationTimestamp(),
		DeletionTimestamp: resource.GetDeletionTimestamp().DeepCopy(),
		// 当前队列仍同时协调状态，保留代次以更新 ObservedGeneration。
		// 后续转发派生结果不应携带代次，避免展示名称修改继续传播到配置输出。
		Generation: resource.GetGeneration(),
	}
}

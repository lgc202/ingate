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

func projectResourceMeta(resource client.Object) metav1.ObjectMeta {
	meta := projectForwardingMeta(resource)
	// Gateway 与 HTTPRoute 当前仍协调状态，保留实例身份和代次用于写回前核对。
	meta.UID = resource.GetUID()
	meta.Generation = resource.GetGeneration()

	return meta
}

func projectForwardingMeta(resource client.Object) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace:         resource.GetNamespace(),
		Name:              resource.GetName(),
		CreationTimestamp: resource.GetCreationTimestamp(),
		DeletionTimestamp: resource.GetDeletionTimestamp().DeepCopy(),
	}
}

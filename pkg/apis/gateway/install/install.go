// Package install 注册 gateway.ingate.io API 到 Kubernetes Scheme
package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	gateway "github.com/lgc202/ingate/pkg/apis/gateway"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Install 将 Gateway API 类型注册到 Scheme
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(gateway.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	utilruntime.Must(scheme.SetVersionPriority(gatewayv1.SchemeGroupVersion))
}

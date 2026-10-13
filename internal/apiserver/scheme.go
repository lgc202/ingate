// Package apiserver 通过 Kubernetes Generic API Server 提供 Namespace、ConfigMap 和声明式网关资源 API。
package apiserver

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	coreinstall "github.com/lgc202/ingate/pkg/apis/core/install"
	gatewayinstall "github.com/lgc202/ingate/pkg/apis/gateway/install"
)

func newScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()

	if err := metav1.AddMetaToScheme(scheme); err != nil {
		return nil, fmt.Errorf("register meta.k8s.io/v1 types: %w", err)
	}
	metav1.AddToGroupVersion(scheme, metav1.SchemeGroupVersion)

	// 仅注册支持的核心资源和标准请求选项，不引入整个 Kubernetes 核心资源集合。
	if err := coreinstall.Install(scheme); err != nil {
		return nil, fmt.Errorf("register core API types: %w", err)
	}

	if err := gatewayinstall.Install(scheme); err != nil {
		return nil, fmt.Errorf("register Gateway API types: %w", err)
	}

	return scheme, nil
}

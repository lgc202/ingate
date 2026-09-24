// Package apiserver 通过 Kubernetes Generic API Server 提供声明式网关资源 API。
package apiserver

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	gatewayinstall "github.com/lgc202/ingate/pkg/apis/gateway/install"
)

func newScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()

	if err := metav1.AddMetaToScheme(scheme); err != nil {
		return nil, fmt.Errorf("register meta.k8s.io/v1 types: %w", err)
	}
	metav1.AddToGroupVersion(scheme, metav1.SchemeGroupVersion)

	// Generic API Server 的请求选项使用核心 v1 编码。
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Version: "v1"})

	if err := gatewayinstall.Install(scheme); err != nil {
		return nil, fmt.Errorf("register Gateway API types: %w", err)
	}

	return scheme, nil
}

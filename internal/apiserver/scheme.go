// Package apiserver 通过 Kubernetes Generic API Server 提供 Namespace、ConfigMap 和声明式网关资源 API。
package apiserver

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	coreinstall "k8s.io/kubernetes/pkg/apis/core/install"

	gatewayinstall "github.com/lgc202/ingate/pkg/apis/gateway/install"
)

func newScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()

	if err := metav1.AddMetaToScheme(scheme); err != nil {
		return nil, fmt.Errorf("register meta.k8s.io/v1 types: %w", err)
	}

	metav1.AddToGroupVersion(scheme, metav1.SchemeGroupVersion)

	// 上游原生存储需要其核心内部类型、转换及默认值；类型注册不安装 REST API。
	// 原生 Strategy 使用上游内置 Scheme，业务类型仍只注册到本实例。
	coreinstall.Install(scheme)

	if err := gatewayinstall.Install(scheme); err != nil {
		return nil, fmt.Errorf("register Gateway API types: %w", err)
	}

	return scheme, nil
}

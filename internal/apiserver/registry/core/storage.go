// Package core 聚合 Ingate 支持的核心资源存储，不注册其它 Kubernetes 核心资源。
package core

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"

	configmapstorage "github.com/lgc202/ingate/internal/apiserver/registry/core/configmap"
	namespacestorage "github.com/lgc202/ingate/internal/apiserver/registry/core/namespace"
)

// NewStorage 创建完整核心存储映射；Namespace 共享存储由主资源负责销毁。
func NewStorage(
	scheme *runtime.Scheme,
	optionsGetter generic.RESTOptionsGetter,
) (map[string]rest.Storage, error) {
	namespaceREST, statusREST, finalizeREST, err := namespacestorage.NewREST(scheme, optionsGetter)
	if err != nil {
		return nil, fmt.Errorf("create Namespace REST storage: %w", err)
	}
	configMapREST, err := configmapstorage.NewREST(scheme, optionsGetter)
	if err != nil {
		namespaceREST.Destroy()
		return nil, fmt.Errorf("create ConfigMap REST storage: %w", err)
	}

	return map[string]rest.Storage{
		"namespaces":          namespaceREST,
		"namespaces/status":   statusREST,
		"namespaces/finalize": finalizeREST,
		"configmaps":          configMapREST,
	}, nil
}

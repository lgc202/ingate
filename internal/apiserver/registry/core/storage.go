// Package core 只安装 Namespace、ConfigMap 存储；类型注册由 API Server 拥有。
package core

import (
	"fmt"

	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	configmapstorage "k8s.io/kubernetes/pkg/registry/core/configmap/storage"
	namespacestorage "k8s.io/kubernetes/pkg/registry/core/namespace/storage"
)

// NewStorage 创建完整核心存储映射；Namespace 共享存储由主资源负责销毁。
func NewStorage(optionsGetter generic.RESTOptionsGetter) (map[string]rest.Storage, error) {
	namespaceREST, statusREST, finalizeREST, err := namespacestorage.NewREST(optionsGetter)
	if err != nil {
		return nil, fmt.Errorf("create Namespace REST storage: %w", err)
	}

	configMapREST, err := configmapstorage.NewREST(optionsGetter)
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

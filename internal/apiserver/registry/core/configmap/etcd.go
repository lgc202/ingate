/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// 来源：Kubernetes v1.37.0 pkg/registry/core/configmap/storage/storage.go。
// 本地适配：注入私有 Scheme；使用公开默认 TableConvertor 和标准 metadata 选择器。
package configmap

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"

	coreapi "github.com/lgc202/ingate/pkg/apis/core"
)

// REST 提供 ConfigMap 的标准 CRUD、Patch、List、Watch 与 DeleteCollection。
type REST struct {
	*genericregistry.Store
}

var (
	_ rest.StandardStorage    = &REST{}
	_ rest.ShortNamesProvider = &REST{}
)

// NewREST 创建 ConfigMap 存储；调用方在使用结束或后续装配失败时负责 Destroy。
func NewREST(
	scheme *runtime.Scheme,
	optionsGetter generic.RESTOptionsGetter,
) (*REST, error) {
	strategy := NewStrategy(scheme)
	resource := coreapi.SchemeGroupVersion.WithResource("configmaps").GroupResource()
	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &coreapi.ConfigMap{} },
		NewListFunc:               func() runtime.Object { return &coreapi.ConfigMapList{} },
		DefaultQualifiedResource:  resource,
		SingularQualifiedResource: coreapi.SchemeGroupVersion.WithResource("configmap").GroupResource(),
		CreateStrategy:            strategy,
		UpdateStrategy:            strategy,
		DeleteStrategy:            strategy,
		TableConvertor:            rest.NewDefaultTableConvertor(resource),
	}

	// ConfigMap 只有标准 metadata 筛选字段，交由 Store 按 NamespaceScoped 自动完成。
	if err := store.CompleteWithOptions(&generic.StoreOptions{RESTOptions: optionsGetter}); err != nil {
		return nil, fmt.Errorf("complete ConfigMap REST storage: %w", err)
	}

	return &REST{Store: store}, nil
}

// ShortNames 返回标准 kubectl 缩写。
func (*REST) ShortNames() []string { return []string{"cm"} }

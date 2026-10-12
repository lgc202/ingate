/*
Copyright 2014 The Kubernetes Authors.

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

// 来源：Kubernetes v1.37.0 pkg/registry/core/namespace/strategy.go 的选择器。
// 本地适配：包内 helper；不保留上游历史 name 字段别名。
package namespace

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"

	coreapi "github.com/lgc202/ingate/pkg/apis/core"
)

func getAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	namespace, ok := obj.(*coreapi.Namespace)
	if !ok {
		return nil, nil, fmt.Errorf("expected *core.Namespace, got %T", obj)
	}

	selectableFields := generic.ObjectMetaFieldsSet(&namespace.ObjectMeta, false)
	selectableFields["status.phase"] = string(namespace.Status.Phase)
	return labels.Set(namespace.Labels), selectableFields, nil
}

func matcher(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{Label: label, Field: field, GetAttrs: getAttrs}
}

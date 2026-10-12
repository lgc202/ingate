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

// 来源：Kubernetes v1.37.0 pkg/apis/core/v1/defaults.go 的 Namespace 默认值。
// 本地适配：合并 Namespace/NamespaceStatus 默认值，仅注册本项目提供的两种对象。
package v1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// SetDefaultsNamespace 补入名称标签和缺省阶段，不重新添加已清除的 finalizer。
func SetDefaultsNamespace(namespace *corev1.Namespace) {
	if namespace.Name != "" {
		if namespace.Labels == nil {
			namespace.Labels = make(map[string]string)
		}
		namespace.Labels[corev1.LabelMetadataName] = namespace.Name
	}
	if namespace.Status.Phase == "" {
		namespace.Status.Phase = corev1.NamespaceActive
	}
}

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	scheme.AddTypeDefaultingFunc(&corev1.Namespace{}, func(obj any) {
		SetDefaultsNamespace(obj.(*corev1.Namespace))
	})
	scheme.AddTypeDefaultingFunc(&corev1.NamespaceList{}, func(obj any) {
		list := obj.(*corev1.NamespaceList)
		for i := range list.Items {
			SetDefaultsNamespace(&list.Items[i])
		}
	})
	return nil
}

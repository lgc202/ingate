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

// 来源：Kubernetes v1.37.0 pkg/apis/core/types.go 的 Namespace 和 NamespaceList。
// 本地适配：嵌套字段复用已发布的 core/v1 类型，不导入 Kubernetes 主仓库。
package core

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Namespace 是命名空间资源的无版本内部对象。
type Namespace struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Spec 保存命名空间删除前必须完成的 finalizer。
	Spec corev1.NamespaceSpec
	// Status 保存生命周期阶段和删除进度。
	Status corev1.NamespaceStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NamespaceList 是命名空间内部对象的列表。
type NamespaceList struct {
	metav1.TypeMeta
	metav1.ListMeta

	Items []Namespace
}

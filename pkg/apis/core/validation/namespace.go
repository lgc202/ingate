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

// 来源：Kubernetes v1.37.0 pkg/apis/core/validation/validation.go 的 Namespace 校验。
// 本地适配：复用 apimachinery；标准 finalizer 名称取自公开常量。
//
// Package validation 定义 Ingate 核心资源的单对象不变量。
package validation

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	coreapi "github.com/lgc202/ingate/pkg/apis/core"
)

// ValidateNamespace 校验集群级 metadata、DNS label 名称和 spec.finalizers。
func ValidateNamespace(namespace *coreapi.Namespace) field.ErrorList {
	errs := apivalidation.ValidateObjectMeta(
		&namespace.ObjectMeta, false, apivalidation.ValidateNamespaceName, field.NewPath("metadata"),
	)
	for i, finalizer := range namespace.Finalizers {
		errs = append(errs, validateStandardFinalizerName(finalizer, field.NewPath("metadata", "finalizers").Index(i))...)
	}
	for _, finalizer := range namespace.Spec.Finalizers {
		errs = append(errs, validateFinalizerName(string(finalizer), field.NewPath("spec", "finalizers"))...)
	}
	return errs
}

// ValidateNamespaceUpdate 校验命名空间 metadata 更新。
func ValidateNamespaceUpdate(next, previous *coreapi.Namespace) field.ErrorList {
	errs := apivalidation.ValidateObjectMetaUpdate(&next.ObjectMeta, &previous.ObjectMeta, field.NewPath("metadata"))
	for i, finalizer := range next.Finalizers {
		errs = append(errs, validateStandardFinalizerName(finalizer, field.NewPath("metadata", "finalizers").Index(i))...)
	}
	return errs
}

// ValidateNamespaceStatusUpdate 校验 metadata 及阶段与删除时间的一致性。
func ValidateNamespaceStatusUpdate(next, previous *coreapi.Namespace) field.ErrorList {
	errs := ValidateNamespaceUpdate(next, previous)
	if next.DeletionTimestamp.IsZero() {
		if next.Status.Phase != corev1.NamespaceActive {
			errs = append(errs, field.Invalid(field.NewPath("status", "phase"), next.Status.Phase,
				"may only be 'Active' if deletionTimestamp is empty"))
		}
		return errs
	}

	if next.Status.Phase != corev1.NamespaceTerminating {
		errs = append(errs, field.Invalid(field.NewPath("status", "phase"), next.Status.Phase,
			"may only be 'Terminating' if deletionTimestamp is not empty"))
	}
	return errs
}

// ValidateNamespaceFinalizeUpdate 校验 metadata 和将写入的 spec.finalizers。
func ValidateNamespaceFinalizeUpdate(next, previous *coreapi.Namespace) field.ErrorList {
	errs := ValidateNamespaceUpdate(next, previous)
	for i, finalizer := range next.Spec.Finalizers {
		errs = append(errs, validateFinalizerName(string(finalizer), field.NewPath("spec", "finalizers").Index(i))...)
	}
	return errs
}

func validateFinalizerName(name string, path *field.Path) field.ErrorList {
	errs := apivalidation.ValidateFinalizerName(name, path)
	return append(errs, validateStandardFinalizerName(name, path)...)
}

func validateStandardFinalizerName(name string, path *field.Path) field.ErrorList {
	if strings.Contains(name, "/") {
		return nil
	}

	switch name {
	case string(corev1.FinalizerKubernetes), metav1.FinalizerOrphanDependents, metav1.FinalizerDeleteDependents:
		return nil
	default:
		return field.ErrorList{field.Invalid(path, name, "name is neither a standard finalizer name nor is it fully qualified")}
	}
}

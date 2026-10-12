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

// 来源：Kubernetes v1.37.0 pkg/registry/core/namespace/strategy.go。
// 本地适配：构造器注入私有 Scheme；字段校验委托 Ingate 的核心 validation 包。
//
// Package namespace 提供标准 Namespace 的存储、特殊删除和三类写入策略。
package namespace

import (
	"context"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	coreapi "github.com/lgc202/ingate/pkg/apis/core"
	corevalidation "github.com/lgc202/ingate/pkg/apis/core/validation"
)

type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

type statusStrategy struct {
	strategy
}

type finalizeStrategy struct {
	strategy
}

var (
	_ rest.RESTCreateStrategy  = strategy{}
	_ rest.RESTUpdateStrategy  = strategy{}
	_ rest.RESTDeleteStrategy  = strategy{}
	_ rest.ResetFieldsStrategy = strategy{}
	_ rest.RESTUpdateStrategy  = statusStrategy{}
	_ rest.ResetFieldsStrategy = statusStrategy{}
	_ rest.RESTUpdateStrategy  = finalizeStrategy{}
	_ rest.ResetFieldsStrategy = finalizeStrategy{}
)

// NewStrategy 创建 Namespace 主资源的写入策略。
func NewStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{ObjectTyper: typer, NameGenerator: names.SimpleNameGenerator}
}

// NewStatusStrategy 创建仅保护 Spec 的标准 Namespace status 策略。
func NewStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: NewStrategy(typer)}
}

// NewFinalizeStrategy 创建仅保护 Status 的标准 Namespace finalize 策略。
func NewFinalizeStrategy(typer runtime.ObjectTyper) finalizeStrategy {
	return finalizeStrategy{strategy: NewStrategy(typer)}
}

// NamespaceScoped 表示 Namespace 本身是集群级资源。
func (strategy) NamespaceScoped() bool {
	return false
}

// GetResetFields 声明主资源入口会重置 Status。
func (strategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"v1": fieldpath.NewSet(fieldpath.MakePathOrDie("status")),
	}
}

// PrepareForCreate 设置 Active，并确保只补入缺失的 kubernetes finalizer。
func (strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	namespace := obj.(*coreapi.Namespace)
	namespace.Status = corev1.NamespaceStatus{Phase: corev1.NamespaceActive}
	if !slices.Contains(namespace.Spec.Finalizers, corev1.FinalizerKubernetes) {
		namespace.Spec.Finalizers = append(namespace.Spec.Finalizers, corev1.FinalizerKubernetes)
	}
}

// Validate 校验待创建的 Namespace。
func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return corevalidation.ValidateNamespace(obj.(*coreapi.Namespace))
}

// WarningsOnCreate 返回创建警告；标准 Namespace 当前没有额外警告。
func (strategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// Canonicalize 在生成名称完成后确保标准 metadata.name 标签与名称一致。
func (strategy) Canonicalize(obj runtime.Object) {
	namespace := obj.(*coreapi.Namespace)
	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}
	namespace.Labels[corev1.LabelMetadataName] = namespace.Name
}

// AllowCreateOnUpdate 禁止通过 Update 创建 Namespace。
func (strategy) AllowCreateOnUpdate(_ context.Context) bool {
	return false
}

// PrepareForUpdate 保留已有的 finalizer 和 Status，不套用网关 Generation 规则。
func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	next := obj.(*coreapi.Namespace)
	previous := old.(*coreapi.Namespace)
	next.Spec.Finalizers = previous.Spec.Finalizers
	next.Status = previous.Status
}

// ValidateUpdate 校验 Namespace 主资源和 metadata 更新。
func (strategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	next := obj.(*coreapi.Namespace)
	previous := old.(*coreapi.Namespace)
	errs := corevalidation.ValidateNamespace(next)
	return append(errs, corevalidation.ValidateNamespaceUpdate(next, previous)...)
}

// WarningsOnUpdate 返回 Namespace 更新警告。
func (strategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

// AllowUnconditionalUpdate 保留标准 Namespace 允许无 resourceVersion 更新的语义。
func (strategy) AllowUnconditionalUpdate(_ context.Context) bool {
	return true
}

// GetResetFields 声明 status 入口会重置 Spec。
func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"v1": fieldpath.NewSet(fieldpath.MakePathOrDie("spec")),
	}
}

// PrepareForUpdate 保留标准 Namespace 的 Spec，不扩展成网关 metadata 重置行为。
func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	next := obj.(*coreapi.Namespace)
	previous := old.(*coreapi.Namespace)
	next.Spec = previous.Spec
}

// ValidateUpdate 校验 Namespace 阶段与删除时间的一致性。
func (statusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return corevalidation.ValidateNamespaceStatusUpdate(obj.(*coreapi.Namespace), old.(*coreapi.Namespace))
}

// GetResetFields 声明 finalize 入口会重置 Status。
func (finalizeStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"v1": fieldpath.NewSet(fieldpath.MakePathOrDie("status")),
	}
}

// PrepareForUpdate 保留已有 Status，允许 finalize 修改 spec.finalizers。
func (finalizeStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	next := obj.(*coreapi.Namespace)
	previous := old.(*coreapi.Namespace)
	next.Status = previous.Status
}

// ValidateUpdate 校验 finalize 的 metadata 和 finalizer。
func (finalizeStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return corevalidation.ValidateNamespaceFinalizeUpdate(obj.(*coreapi.Namespace), old.(*coreapi.Namespace))
}

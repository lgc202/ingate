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

// 来源：Kubernetes v1.37.0 pkg/registry/core/configmap/strategy.go。
// 本地适配：注入私有 ObjectTyper；保留显式字段校验，不接入主仓库 DeclarativeValidation。
//
// Package configmap 提供标准 ConfigMap 存储，不解释应用配置或管理数据面发布。
package configmap

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"

	coreapi "github.com/lgc202/ingate/pkg/apis/core"
	corevalidation "github.com/lgc202/ingate/pkg/apis/core/validation"
)

type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

var (
	_ rest.RESTCreateStrategy = strategy{}
	_ rest.RESTUpdateStrategy = strategy{}
	_ rest.RESTDeleteStrategy = strategy{}
)

// NewStrategy 创建 ConfigMap 写入策略，不套用网关 Spec/Status 或 Generation 语义。
func NewStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{ObjectTyper: typer, NameGenerator: names.SimpleNameGenerator}
}

// NamespaceScoped 表示 ConfigMap 属于命名空间。
func (strategy) NamespaceScoped() bool { return true }

// PrepareForCreate 保留 ConfigMap 内容，没有需要重置的 Spec 或 Status。
func (strategy) PrepareForCreate(_ context.Context, _ runtime.Object) {}

// Validate 校验标准 metadata、配置 key 和总数据大小。
func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return corevalidation.ValidateConfigMap(obj.(*coreapi.ConfigMap))
}

// WarningsOnCreate 返回创建警告；当前没有弃用字段。
func (strategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string { return nil }

// Canonicalize 保留 ConfigMap 数据的原值，不改写应用配置。
func (strategy) Canonicalize(_ runtime.Object) {}

// AllowCreateOnUpdate 禁止通过普通 Update 创建 ConfigMap。
func (strategy) AllowCreateOnUpdate(_ context.Context) bool { return false }

// PrepareForUpdate 保留提交的数据，不维护业务资源的 Generation。
func (strategy) PrepareForUpdate(_ context.Context, _, _ runtime.Object) {}

// ValidateUpdate 校验 metadata 更新与标准 immutable 限制。
func (strategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return corevalidation.ValidateConfigMapUpdate(obj.(*coreapi.ConfigMap), old.(*coreapi.ConfigMap))
}

// WarningsOnUpdate 返回更新警告；当前没有弃用字段。
func (strategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string { return nil }

// AllowUnconditionalUpdate 沿用标准 ConfigMap 的无条件更新；配置发布者仍应携带 resourceVersion。
func (strategy) AllowUnconditionalUpdate(_ context.Context) bool { return true }

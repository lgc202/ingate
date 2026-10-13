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

// 来源：Kubernetes v1.37.0 pkg/apis/core/validation/validation.go 的 ConfigMap 校验。
// 本地适配：复用公开 metadata/key 校验与大小常量，不导入 Kubernetes 主仓库。
package validation

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	coreapi "github.com/lgc202/ingate/pkg/apis/core"
)

// ValidateConfigMap 校验命名空间 metadata、key 语法、重复 key 和标准总数据上限。
func ValidateConfigMap(configMap *coreapi.ConfigMap) field.ErrorList {
	errs := apivalidation.ValidateObjectMeta(
		&configMap.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"),
	)
	for i, finalizer := range configMap.Finalizers {
		errs = append(errs, validateStandardFinalizerName(finalizer, field.NewPath("metadata", "finalizers").Index(i))...)
	}

	totalSize := 0
	for key, value := range configMap.Data {
		for _, message := range validation.IsConfigMapKey(key) {
			errs = append(errs, field.Invalid(field.NewPath("data").Key(key), key, message))
		}
		if _, exists := configMap.BinaryData[key]; exists {
			errs = append(errs, field.Invalid(field.NewPath("data").Key(key), key, "duplicate of key present in binaryData"))
		}
		totalSize += len(value)
	}
	for key, value := range configMap.BinaryData {
		for _, message := range validation.IsConfigMapKey(key) {
			errs = append(errs, field.Invalid(field.NewPath("binaryData").Key(key), key, message))
		}
		totalSize += len(value)
	}
	if totalSize > corev1.MaxSecretSize {
		errs = append(errs, field.TooLong(field.NewPath(""), "", corev1.MaxSecretSize))
	}
	return errs
}

// ValidateConfigMapUpdate 校验 metadata 及已设置 immutable 的内容保护；仍允许更新合法 metadata。
func ValidateConfigMapUpdate(next, previous *coreapi.ConfigMap) field.ErrorList {
	errs := apivalidation.ValidateObjectMetaUpdate(&next.ObjectMeta, &previous.ObjectMeta, field.NewPath("metadata"))
	if previous.Immutable != nil && *previous.Immutable {
		if next.Immutable == nil || !*next.Immutable {
			errs = append(errs, field.Forbidden(field.NewPath("immutable"), "field is immutable when immutable is set"))
		}
		if !reflect.DeepEqual(next.Data, previous.Data) {
			errs = append(errs, field.Forbidden(field.NewPath("data"), "field is immutable when immutable is set"))
		}
		if !reflect.DeepEqual(next.BinaryData, previous.BinaryData) {
			errs = append(errs, field.Forbidden(field.NewPath("binaryData"), "field is immutable when immutable is set"))
		}
	}

	return append(errs, ValidateConfigMap(next)...)
}

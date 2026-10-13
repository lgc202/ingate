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

// 来源：Kubernetes v1.37.0 pkg/apis/core/v1/conversion.go 的 Namespace 字段转换。
// 本地适配：只追加 Namespace 特定的字段转换；ConfigMap 使用 Scheme 默认的 metadata 字段转换。
package v1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func addFieldLabelConversions(scheme *runtime.Scheme) error {
	return scheme.AddFieldLabelConversionFunc(corev1.SchemeGroupVersion.WithKind("Namespace"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", "status.phase":
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		})
}

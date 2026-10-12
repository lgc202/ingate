// Package install 显式注册 Ingate 提供的核心资源类型和转换。
package install

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lgc202/ingate/pkg/apis/core"
	"github.com/lgc202/ingate/pkg/apis/core/v1"
)

// Install 注册 Namespace 内外部类型、转换和默认值，并指定核心 v1 为首选外部版本。
func Install(scheme *runtime.Scheme) error {
	if err := core.AddToScheme(scheme); err != nil {
		return err
	}
	if err := v1.AddToScheme(scheme); err != nil {
		return err
	}

	return scheme.SetVersionPriority(corev1.SchemeGroupVersion)
}

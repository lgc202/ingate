// Package install 将 Gateway API 的内部类型和对外版本注册到 Scheme。
package install

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lgc202/ingate/pkg/apis/gateway"
	"github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// Install 注册 Gateway API 的类型和转换，并指定首选对外版本。
func Install(scheme *runtime.Scheme) error {
	if err := gateway.AddToScheme(scheme); err != nil {
		return err
	}

	if err := v1alpha1.AddToScheme(scheme); err != nil {
		return err
	}

	return scheme.SetVersionPriority(v1alpha1.SchemeGroupVersion)
}

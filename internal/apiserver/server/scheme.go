// Package server 提供 ingate-apiserver 的 Kubernetes generic apiserver 基础配置
package server

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	gatewayinstall "github.com/lgc202/ingate/internal/pkg/apis/gateway/install"
)

var (
	// Scheme 保存 ingate-apiserver 支持的 API 类型
	Scheme = newScheme()
	// Codecs 提供 Kubernetes API 编解码器
	Codecs = serializer.NewCodecFactory(Scheme)
)

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	gatewayinstall.Install(scheme)

	// discovery、watch 和错误响应使用 Kubernetes 的无组版本元类型
	unversionedVersion := schema.GroupVersion{Group: "", Version: "v1"}
	metav1.AddToGroupVersion(scheme, unversionedVersion)
	scheme.AddUnversionedTypes(unversionedVersion,
		&metav1.Status{},
		&metav1.WatchEvent{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)
	return scheme
}

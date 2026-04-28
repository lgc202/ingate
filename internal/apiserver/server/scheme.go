// Package server 提供 ingate-apiserver 的 Kubernetes generic apiserver 基础配置
package server

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	gatewayinstall "github.com/lgc202/ingate-next/pkg/apis/gateway/install"
)

var (
	// Scheme 保存 ingate-apiserver 支持的 API 类型
	Scheme = runtime.NewScheme()
	// Codecs 提供 Kubernetes API 编解码器
	Codecs = serializer.NewCodecFactory(Scheme)
)

var unversionedVersion = schema.GroupVersion{Group: "", Version: "v1"}

var unversionedTypes = []runtime.Object{
	&metav1.Status{},
	&metav1.WatchEvent{},
	&metav1.APIVersions{},
	&metav1.APIGroupList{},
	&metav1.APIGroup{},
	&metav1.APIResourceList{},
}

func init() {
	gatewayinstall.Install(Scheme)
	metav1.AddToGroupVersion(Scheme, unversionedVersion)
	Scheme.AddUnversionedTypes(unversionedVersion, unversionedTypes...)
}

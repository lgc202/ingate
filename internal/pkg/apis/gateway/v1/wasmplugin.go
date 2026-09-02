package v1

import (
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// WasmPluginPackageTransformer 标识 Ingate 请求响应转换标准插件。
	WasmPluginPackageTransformer = "ingate-transformer"
	// WasmPluginPackageMockResponse 标识 Ingate 模拟响应标准插件。
	WasmPluginPackageMockResponse = "ingate-mock-response"
)

// WasmPluginPullPolicy 表示控制面何时重新拉取 Wasm 模块。
type WasmPluginPullPolicy string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced

const (
	// WasmPluginPullIfNotPresent 优先复用校验和一致的本地模块。
	WasmPluginPullIfNotPresent WasmPluginPullPolicy = "IfNotPresent"
	// WasmPluginPullAlways 在资源版本变化时重新拉取模块。
	WasmPluginPullAlways WasmPluginPullPolicy = "Always"
)

// WasmPlugin 声明一个已安装的 Proxy-Wasm 插件制品。
type WasmPlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WasmPluginSpec `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// WasmPluginList 表示 WasmPlugin 资源列表。
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WasmPluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []WasmPlugin `json:"items"`
}

// WasmPluginSpec 定义插件身份和模块来源。
type WasmPluginSpec struct {
	// SourceID 标识安装插件时选中的插件源，升级始终回到同一来源解析制品。
	SourceID string `json:"sourceID"`
	// DisplayName 保存控制台展示名称，不参与流量匹配。
	DisplayName string `json:"displayName"`
	// Package 是插件在仓库中的稳定标识，同一配置域只能安装一个版本。
	Package string `json:"package"`
	// Version 表示插件制品版本，不等同于资源 Generation。
	Version string `json:"version"`
	// URL 支持指向 Wasm 模块的 HTTP(S) URL 或 oci:// 镜像引用。
	URL string `json:"url"`
	// SHA256 对 HTTP(S) 校验 Wasm 模块，对 OCI 校验镜像 manifest。
	SHA256 string `json:"sha256"`
	// PullPolicy 控制资源版本变化时是否重新拉取模块。
	PullPolicy WasmPluginPullPolicy `json:"pullPolicy"`
	// RootID 必须与 Proxy-Wasm 模块注册 Context 时使用的 root_id 一致；
	// 为空表示使用模块的默认 Root Context。
	RootID string `json:"rootID,omitempty"`
}

// SupportedWasmPluginPackages 返回当前控制器具有强类型策略适配器的插件包。
// 插件源可以独立分发这些包的新版本，
// 但不能仅靠目录文件引入控制器不理解的新策略语义。
func SupportedWasmPluginPackages() []string {
	return []string{
		WasmPluginPackageTransformer,
		WasmPluginPackageMockResponse,
	}
}

// IsSupportedWasmPluginPackage 判断插件包是否具有完整的安装、策略和编译链路。
func IsSupportedWasmPluginPackage(packageName string) bool {
	return slices.Contains(SupportedWasmPluginPackages(), packageName)
}

package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced

// PluginSource 声明一个由用户维护的远程插件目录
// 官方目录由进程配置提供，不重复写入声明式资源
type PluginSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PluginSourceSpec `json:"spec,omitempty"`
	Status ResourceStatus   `json:"status,omitempty"`
}

// PluginSourceList 表示 PluginSource 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PluginSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PluginSource `json:"items"`
}

// PluginSourceSpec 定义插件目录的用户配置
type PluginSourceSpec struct {
	// DisplayName 保存控制台展示名称
	DisplayName string `json:"displayName"`
	// URL 指向与官方目录使用相同结构的 JSON 文件
	URL string `json:"url"`
	// Enabled 控制该目录是否参与插件发现和升级
	Enabled bool `json:"enabled"`
}

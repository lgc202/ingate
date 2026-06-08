package gateway

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RuntimeSnapshot 表示 controller 编译后交给运行时 target 的配置快照
type RuntimeSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RuntimeSnapshotSpec `json:"spec,omitempty"`
}

// RuntimeSnapshotList 表示 RuntimeSnapshot 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RuntimeSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RuntimeSnapshot `json:"items"`
}

// RuntimeSnapshotSpec 定义某个 target 可消费的网关配置快照
type RuntimeSnapshotSpec struct {
	Target  string               `json:"target"`
	Gateway string               `json:"gateway"`
	Version string               `json:"version"`
	Config  runtime.RawExtension `json:"config"`
}

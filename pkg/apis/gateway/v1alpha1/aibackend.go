package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +resourceName=aibackends
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AIBackend 为 Backend 声明模型服务的上游 API 格式，不保存模型映射或凭据。
type AIBackend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AIBackendSpec   `json:"spec"`
	Status AIBackendStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AIBackendList 表示 AIBackend 集合。
type AIBackendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AIBackend `json:"items"`
}

// AIBackendSpec 定义模型服务的期望访问契约。
type AIBackendSpec struct {
	// DisplayName 是可修改且允许重复的展示名称，不作为资源引用依据。
	DisplayName string `json:"displayName,omitempty"`
	// BackendRef 引用集群级 Backend，网络目标由该资源维护。
	BackendRef BackendReference `json:"backendRef"`
	// Schema 声明上游 API 格式；保存该配置不表示数据面已经实现转发。
	Schema APISchema `json:"schema"`
}

// APISchema 定义上游 API 格式及路径前缀，首版仅支持 OpenAI。
type APISchema struct {
	// Name 是必填的 API 格式名称，不是 Chat Completions 等操作名称。
	Name APISchemaName `json:"name"`
	// Prefix 是可带或不带前导斜杠的路径前缀；省略或空值默认 /v1。
	Prefix string `json:"prefix,omitempty"`
}

// APISchemaName 表示模型服务使用的 API 格式名称。
type APISchemaName string

const (
	// APISchemaOpenAI 表示 OpenAI API 格式，不承诺支持该格式的全部操作。
	APISchemaOpenAI APISchemaName = "OpenAI"
)

// AIBackendStatus 保存协调器的观测结果，不由资源写入操作填充。
type AIBackendStatus struct {
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

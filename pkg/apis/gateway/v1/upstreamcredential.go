package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// UpstreamCredentialType 表示访问上游服务时使用的凭据类型
type UpstreamCredentialType string

const (
	// UpstreamCredentialTypeAPIKey 表示静态 API Key 凭据
	UpstreamCredentialTypeAPIKey UpstreamCredentialType = "APIKey"
)

// APIKeyCredential 保存静态 API Key
type APIKeyCredential struct {
	// Value 保存发送给上游服务的完整 API Key
	Value string `json:"value"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced

// UpstreamCredential 声明 Ingate 访问上游服务时使用的凭据
type UpstreamCredential struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UpstreamCredentialSpec `json:"spec,omitempty"`
	Status ResourceStatus         `json:"status,omitempty"`
}

// UpstreamCredentialList 表示 UpstreamCredential 资源列表
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UpstreamCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []UpstreamCredential `json:"items"`
}

// UpstreamCredentialSpec 定义上游访问凭据的展示名称、类型和密钥内容
type UpstreamCredentialSpec struct {
	// DisplayName 保存控制台展示名称，不参与资源引用
	DisplayName string `json:"displayName,omitempty"`
	// Type 指定凭据类型
	Type UpstreamCredentialType `json:"type,omitempty"`
	// APIKey 保存 APIKey 类型凭据的密钥内容
	APIKey *APIKeyCredential `json:"apiKey,omitempty"`
}
